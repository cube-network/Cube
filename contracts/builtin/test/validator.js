const hre = require("hardhat");
const {BigNumber} = require("ethers");
const {expect} = require("chai");
const ethers = hre.ethers;
const utils = require("./utils");

const State = {
    Idle: 0,
    Ready: 1,
    Jail: 2,
    Exit: 3
}

const params = {
    MaxStakes: 24000000,
    OverMaxStakes: 24000001,
    ThresholdStakes: 2000000,
    MinSelfStakes: 150000,
    StakeUnit: 1,
}

describe("Validator test", function () {
    let signers
    let owner
    let factory
    let vSigner; // validator
    let vaddr; // validator address
    let validator // validator contract

    let commissionRate = 50;
    let currTotalStakeGwei;

    before(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        factory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);
        let stake = utils.ethToGwei(params.MinSelfStakes);
        currTotalStakeGwei = stake;
        validator = await factory.deploy(vaddr, vaddr, commissionRate, stake, true, State.Idle);
        expect(validator.address).to.be.properAddress
    });

    it('should has correct owner and manager', async () => {
        await expect(await validator.owner()).to.be.eq(owner.address);
        await expect(await validator.admin()).to.be.eq(vaddr);
    });

    it('should check maxStakes at deploy', async () => {
        let vaddr = vSigner.address;
        let stake = utils.ethToGwei(params.OverMaxStakes);
        await expect(factory.deploy(vaddr, vaddr, commissionRate, stake, true, State.Ready)).to.be.reverted;
    });

    it('1. the state should be ready when there is enough stakes, and the rewards and commission etc. are all correct', async () => {
        // send 2 * params.MinSelfStakes wei as rewards, then the accRewardsPerStake should be 1,
        // and selfDebt should be params.ThresholdStakes
        let delta = utils.ethToGwei(params.ThresholdStakes)
        await expect(validator.addStake(delta, {value: utils.ethToGwei(2 * params.MinSelfStakes)})).to
            .emit(validator, "StateChanged")
            .withArgs(vaddr, vaddr, State.Idle, State.Ready);
        currTotalStakeGwei = currTotalStakeGwei.add(delta)
        expect(await validator.state()).eq(State.Ready);
        expect(await validator.totalStake()).eq(currTotalStakeGwei);
        expect(await validator.accRewardsPerStake()).eq(1);
        expect(await validator.currCommission()).eq(utils.ethToGwei(params.MinSelfStakes));
        expect(await validator.anyClaimable(0, vaddr)).eq(utils.ethToGwei(2 * params.MinSelfStakes));
    });

    it('2. should correct for validatorClaimAny', async () => {
        await expect(validator.validatorClaimAny(vaddr)).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, vaddr, utils.ethToGwei(2 * params.MinSelfStakes));
    });

    it('3. should add delegation and calc rewards correctly', async () => {
        // add delta as delegation, and send 2*`currTotalStakeGwei` of wei as rewards;
        // then the new added delegation should not have any rewards;
        // and the validator can get all the rewards.
        let delta = utils.ethToGwei(params.ThresholdStakes)
        let delegator = signers[2].address;
        let halfRewards = currTotalStakeGwei
        await expect(validator.addDelegation(delta, delegator, {value: halfRewards.mul(2)})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.add(delta));
        currTotalStakeGwei = currTotalStakeGwei.add(delta);
        expect(await validator.state()).eq(State.Ready);
        expect(await validator.totalStake()).eq(currTotalStakeGwei);
        expect(await validator.accRewardsPerStake()).eq(2);
        expect(await validator.currCommission()).eq(halfRewards);
        expect(await validator.anyClaimable(0, vaddr)).eq(halfRewards.mul(2));
        // currently the delegator has no rewards
        expect(await validator.anyClaimable(0, delegator)).eq(0);
        // validator claim
        await expect(validator.validatorClaimAny(vaddr)).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, vaddr, halfRewards.mul(2));
    });

    it('4. should correct for delegatorClaimAny', async () => {
        let rewards = currTotalStakeGwei.mul(2);
        let delegatorRewards = utils.ethToGwei(params.ThresholdStakes);
        let delegator = signers[2].address;
        // console.log(await validator.delegators(delegator));
        await expect(validator.delegatorClaimAny(delegator, {value: rewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, delegator, delegatorRewards);
    });

})

describe("Validator independent test", function () {
    let signers
    let owner
    let factory
    let validator // contract
    let vSigner; // validator
    let vaddr; // validator address
    let delegator // address

    let commissionRate = 50;
    let currTotalStakeGwei;
    let stake = utils.ethToGwei(500000);

    beforeEach(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        delegator = signers[2].address;
        factory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);
        validator = await factory.deploy(vaddr, vaddr, commissionRate, stake, true, State.Idle);
        await validator.addDelegation(stake, delegator);
        currTotalStakeGwei = stake.mul(2);
    });


    it('subStake with correct rewards calculation', async () => {
        // subStake
        // current total stake: 1m , validator: 500k, delegator 500k
        // validator subtract 100k,
        // ==> 900k, 400k, 500k
        let delta = utils.ethToGwei(100000);
        await expect(validator.subStake(delta)).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, vaddr, currTotalStakeGwei.sub(delta));
        // and then settle rewards , set rewards to 2 * 900k
        let currTotalRewards = currTotalStakeGwei.sub(delta).mul(2);
        // validator commission: 50% ==> 900k
        // validator rewards 4/9 ==> 400k
        // delegator rewards 5/9 ==> 500k
        let valExpectRewards = currTotalRewards.div(18).mul(13);
        let delegatorExpectRewards = currTotalRewards.sub(valExpectRewards);
        expect(await validator.anyClaimable(currTotalRewards, vaddr)).eq(valExpectRewards);
        await expect(validator.validatorClaimAny(vaddr, {value: currTotalRewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, vaddr, valExpectRewards);
        // the delegator has half currTotalRewards as staking rewards
        expect(await validator.anyClaimable(0, delegator)).eq(delegatorExpectRewards);
    });

    it('subDelegation with correct rewards calculation', async () => {
        // subDelegation with rewards
        // current total stake: 1m , validator: 500k, delegator 500k
        // delegator subtract 500k,
        // ==> 500k, 500k, 0
        let delta = utils.ethToGwei(500000);
        // currTotalRewards 2m
        let settledRewards = currTotalStakeGwei.mul(2);
        //
        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta));
        // currently ,the delegator should has 1/4 of settledRewards;
        // and it can't share the later rewards
        let delegatorExpectRewards = settledRewards.div(4);
        expect(await validator.anyClaimable(settledRewards, delegator)).eq(delegatorExpectRewards);

        // double rewards ==> commission: 2m, validator: 500k + 1m = 1.5m , that is 7/8 of total rewards, delegator: 500k + 0 = 500k, 1/8 total rewards
        let validatorExpectRewards = settledRewards.mul(2*7).div(8)
        await expect(validator.validatorClaimAny(vaddr, {value: settledRewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, vaddr, validatorExpectRewards);

        expect(await validator.anyClaimable(0, delegator)).eq(delegatorExpectRewards);
    });

})