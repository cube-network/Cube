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
    LazyPunishFactor: 1,
    EvilPunishFactor: 10,
    PunishBase: 1000,
}

describe("Validator test", function () {
    let signers
    let owner
    let factory
    let vSigner; // validator
    let vaddr; // validator address
    let adminSigner; // admin signer
    let adminAddr; // admin address
    let validator // validator contract

    let commissionRate = 50;
    let currTotalStakeGwei;
    let initStake = utils.ethToGwei(params.MinSelfStakes);
    let initAcceptDelegation = true;
    let initState = State.Idle;

    before(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        adminSigner = signers[2];
        adminAddr = adminSigner.address;
        factory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);
        currTotalStakeGwei = initStake;
        validator = await factory.deploy(vaddr, adminAddr, commissionRate, initStake, initAcceptDelegation, initState);
    });

    it('should check invalid parameter at deploy', async () => {
        await expect(factory.deploy(0, vaddr, commissionRate, initStake, true, State.Ready)).to.be.reverted;
        await expect(factory.deploy(vaddr, 0, commissionRate, initStake, true, State.Ready)).to.be.reverted;
        await expect(factory.deploy(vaddr, vaddr, 101, initStake, true, State.Ready)).to.be.reverted;
        let stake = utils.ethToGwei(params.OverMaxStakes);
        await expect(factory.deploy(vaddr, vaddr, commissionRate, stake, true, State.Ready)).to.be.reverted;
    });

    it('Initialization parameter check', async () => {
        expect(validator.address).to.be.properAddress
        expect(await validator.owner()).eq(owner.address);
        expect(await validator.validator()).eq(vaddr);
        expect(await validator.admin()).eq(adminAddr);
        expect(await validator.commissionRate()).eq(commissionRate);
        expect(await validator.selfStakeGWei()).eq(initStake);
        expect(await validator.totalStake()).eq(initStake);
        expect(await validator.totalUnWithdrawn()).eq(initStake);
        expect(await validator.acceptDelegation()).eq(initAcceptDelegation);
        expect(await validator.state()).eq(initState);
    });

    it('1. the state should be ready when there is enough stakes, and the rewards and commission etc. are all correct', async () => {
        // send 2 * params.MinSelfStakes wei as rewards, then the accRewardsPerStake should be 1,
        // and selfDebt should be params.ThresholdStakes
        let delta = utils.ethToGwei(params.ThresholdStakes)
        let sendRewardsWei = utils.ethToWei(2 * params.MinSelfStakes)
        let sendRewardsGwei = utils.ethToGwei(2 * params.MinSelfStakes)
        let oldTotalStake = await validator.totalStake()
        let oldAccRewardsPerStake = await validator.accRewardsPerStake();

        await expect(validator.addStake(delta, {value: sendRewardsGwei})).to
            .emit(validator, "StateChanged")
            .withArgs(vaddr, adminAddr, State.Idle, State.Ready);
        let balance = await ethers.provider.getBalance(validator.address);
        expect(balance).eq(sendRewardsGwei);

        // handleReceivedRewards()
        let receivedRewards = TestHandleReceivedRewards(sendRewardsGwei, oldAccRewardsPerStake, commissionRate, oldTotalStake)
        let accRewardsPerStake = receivedRewards.accRewardsPerStake;
        let currCommission = receivedRewards.currCommission;

        currTotalStakeGwei = currTotalStakeGwei.add(delta)
        expect(await validator.state()).eq(State.Ready);
        expect(await validator.totalStake()).eq(currTotalStakeGwei);
        expect(await validator.totalUnWithdrawn()).eq(currTotalStakeGwei);
        expect(await validator.accRewardsPerStake()).eq(accRewardsPerStake);
        expect(await validator.selfStakeGWei()).eq(currTotalStakeGwei);
        expect(await validator.getSelfDebt()).eq(accRewardsPerStake*delta);
        expect(await validator.currCommission()).eq(currCommission);
        expect(await validator.anyClaimable(0, adminAddr)).eq(sendRewardsGwei);
    });

    it('2. should correct for validatorClaimAny', async () => {
        let sendRewards = utils.ethToGwei(2 * params.MinSelfStakes)

        let accRewardsPerStake = await validator.accRewardsPerStake()
        let selfStakeGWei = await validator.selfStakeGWei()
        let selfDebt= await validator.getSelfDebt()
        let selfSettledRewards = await validator.getSelfSettledRewards()
        let currCommission = await validator.currCommission()
        let currFeeRewards = await validator.currFeeRewards()
        let stakingRewards = accRewardsPerStake.mul(selfStakeGWei).add(selfSettledRewards).sub(selfDebt)
        let rewardsGwei = stakingRewards.add(currCommission).add(currFeeRewards)
        await expect(validator.validatorClaimAny(adminAddr)).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, adminAddr, rewardsGwei);
    });

    it('3. should add delegation and calc rewards correctly', async () => {
        // add delta as delegation, and send 2*`currTotalStakeGwei` of wei as rewards;
        // then the new added delegation should not have any rewards;
        // and the validator can get all the rewards.
        let delta = utils.ethToGwei(params.ThresholdStakes)
        let delegator = signers[3].address;
        let halfRewards = currTotalStakeGwei
        let sendRewardsGwei = halfRewards.mul(2)
        let oldTotalStake = await validator.totalStake()
        let oldAccRewardsPerStake = await validator.accRewardsPerStake();

        await expect(validator.addDelegation(delta, delegator, {value: sendRewardsGwei})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.add(delta));
        currTotalStakeGwei = currTotalStakeGwei.add(delta);
        expect(await validator.state()).eq(State.Ready);
        expect(await validator.totalStake()).eq(currTotalStakeGwei);


        let receivedRewards = TestHandleReceivedRewards(sendRewardsGwei, oldAccRewardsPerStake, commissionRate, oldTotalStake)
        let accRewardsPerStake = receivedRewards.accRewardsPerStake;
        let currCommission = receivedRewards.currCommission;

        expect(await validator.accRewardsPerStake()).eq(accRewardsPerStake);
        expect(await validator.currCommission()).eq(currCommission);
        expect(await validator.anyClaimable(0, adminAddr)).eq(halfRewards.mul(2));
        // currently the delegator has no rewards
        let dlg = await validator.delegators(delegator);
        let claimableGwei = accRewardsPerStake.mul(dlg.stakeGWei).add(dlg.settled).sub(dlg.debt);
        expect(await validator.anyClaimable(0, delegator)).eq(claimableGwei);
        // validator claim
        await expect(validator.validatorClaimAny(adminAddr)).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, adminAddr, sendRewardsGwei);
    });

    it('4. should correct for delegatorClaimAny', async () => {
        let rewards = currTotalStakeGwei.mul(2);
        let delegatorRewardsGwei = utils.ethToGwei(params.ThresholdStakes);
        let delegator = signers[3].address;
        // console.log(await validator.delegators(delegator));
        await expect(validator.delegatorClaimAny(delegator, {value: rewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, delegator, delegatorRewardsGwei);
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
    let adminSigner; // admin signer
    let adminAddr; // admin address

    let commissionRate = 50;
    let currTotalStakeGwei;
    let stake = utils.ethToGwei(500000);

    beforeEach(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        adminSigner = signers[2];
        adminAddr = adminSigner.address;
        delegator = signers[3].address;

        factory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);
        validator = await factory.deploy(vaddr, adminAddr, commissionRate, stake, true, State.Idle);
        await validator.addDelegation(stake, delegator);
        currTotalStakeGwei = stake.mul(2);
    });

    it('1. subStake with correct rewards calculation', async () => {
        // subStake
        // current total stake: 1m , validator: 500k, delegator 500k
        // validator subtract 100k,
        // ==> 900k, 400k, 500k
        expect(await validator.totalStake()).eq(currTotalStakeGwei);
        let selfStakeGWei = await validator.selfStakeGWei();
        expect(selfStakeGWei).eq(stake);
        if (currTotalStakeGwei >= utils.ethToGwei(params.ThresholdStakes) && selfStakeGWei >= utils.ethToGwei(params.MinSelfStakes)) {
            expect(await validator.state()).eq(State.Ready);
        } else {
            expect(await validator.state()).eq(State.Idle);
        }
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(0);

        let delta = utils.ethToGwei(100000); // 100000000000000 wei
        await expect(validator.subStake(delta)).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, adminAddr, currTotalStakeGwei.sub(delta));
        // and then settle rewards , set rewards to 2 * 900k
        let currTotalRewards = currTotalStakeGwei.sub(delta).mul(2);
        // validator commission: 50% ==> 900k
        // validator rewards 4/9 ==> 400k
        // delegator rewards 5/9 ==> 500k
        let valExpectRewards = currTotalRewards.div(18).mul(13);
        let delegatorExpectRewards = currTotalRewards.sub(valExpectRewards);

        expect(await validator.testGetClaimableUnbound(vaddr)).eq(delta);
        expect(await validator.anyClaimable(currTotalRewards, adminAddr)).eq(valExpectRewards.add(utils.gweiToWei(delta)));
        await expect(validator.validatorClaimAny(adminAddr, {value: currTotalRewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, adminAddr, valExpectRewards);
        // the delegator has half currTotalRewards as staking rewards
        expect(await validator.anyClaimable(0, delegator)).eq(delegatorExpectRewards);
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(0);

    });

    it('2. subDelegation with correct rewards calculation', async () => {
        // subDelegation with rewards
        // current total stake: 1m , validator: 500k, delegator 500k
        // delegator subtract 500k,
        // ==> 500k, 500k, 0
        let delta = utils.ethToGwei(500000);
        // currTotalRewards 2m
        let settledRewards = currTotalStakeGwei.mul(2);

        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta);

        // currently ,the delegator should has 1/4 of settledRewards;
        // and it can't share the later rewards
        let delegatorExpectRewards = settledRewards.div(4);
        expect(await validator.anyClaimable(settledRewards, delegator)).eq(delegatorExpectRewards.add(utils.gweiToWei(delta)));

        // double rewards ==> commission: 2m, validator: 500k + 1m = 1.5m , that is 7/8 of total rewards, delegator: 500k + 0 = 500k, 1/8 total rewards
        let validatorExpectRewards = settledRewards.mul(2*7).div(8)
        await expect(validator.validatorClaimAny(adminAddr, {value: settledRewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, adminAddr, validatorExpectRewards);
        expect(await validator.anyClaimable(0, delegator)).eq(delegatorExpectRewards.add(utils.gweiToWei(delta)));

        await expect(validator.delegatorClaimAny(delegator, {value: 0})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, delegator, delegatorExpectRewards);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);
    });

    it('3. exitStaking with correct rewards calculation', async () => {
        let oldTotalStake = await validator.totalStake();
        let oldSelfStakeGWei = await validator.selfStakeGWei();
        let sendRewardsGwei = currTotalStakeGwei.mul(2);

        let oldAccRewardsPerStake = await validator.accRewardsPerStake();
        let receivedRewards = TestHandleReceivedRewards(sendRewardsGwei, oldAccRewardsPerStake, commissionRate, oldTotalStake)
        let accRewardsPerStake = receivedRewards.accRewardsPerStake;
        let currCommission = receivedRewards.currCommission;

        expect(await validator.state()).eq(State.Idle);
        await expect(validator.exitStaking({value: sendRewardsGwei})).to
            .emit(validator, "StateChanged")
            .withArgs(vaddr, adminAddr, State.Idle, State.Exit);
        expect(await validator.state()).eq(State.Exit);
        expect(await validator.totalStake()).eq(oldTotalStake.sub(oldSelfStakeGWei));
        expect(await validator.accRewardsPerStake()).eq(accRewardsPerStake);
        expect(await validator.currCommission()).eq(currCommission);
    });

    it('4. exitDelegation with correct rewards calculation', async () => {
        let oldTotalStake = await validator.totalStake();
        let sendRewardsGwei = currTotalStakeGwei.mul(2);
        let oldAccRewardsPerStake = await validator.accRewardsPerStake();
        let receivedRewards = TestHandleReceivedRewards(sendRewardsGwei, oldAccRewardsPerStake, commissionRate, oldTotalStake)
        let accRewardsPerStake = receivedRewards.accRewardsPerStake;
        let currCommission = receivedRewards.currCommission;

        let dlg = await validator.delegators(delegator);
        let oldStakeGWei = dlg.stakeGWei;

        let oldPendingUnbound = await validator.testGetClaimableUnbound(delegator);
        expect(oldPendingUnbound).eq(0);

        await expect(validator.exitDelegation(delegator, {value: sendRewardsGwei})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, oldTotalStake.sub(oldStakeGWei));
        expect(await validator.accRewardsPerStake()).eq(accRewardsPerStake);
        expect(await validator.currCommission()).eq(currCommission);
        expect(await validator.totalStake()).eq(oldTotalStake.sub(oldStakeGWei));

        let newDlg = await validator.delegators(delegator);
        expect(newDlg.settled).eq(oldStakeGWei * accRewardsPerStake);
        expect(newDlg.stakeGWei).eq(0);

        //console.log(await validator.getPendingUnboundRecord(delegator, 0));
        let newPendingUnbound = await validator.testGetClaimableUnbound(delegator);
        expect(newPendingUnbound).eq(oldStakeGWei);

        dlg = await validator.delegators(delegator);
        let claimableGwei = accRewardsPerStake.mul(dlg.stakeGWei).add(dlg.settled).sub(dlg.debt);
        await expect(validator.delegatorClaimAny(delegator, {value: 0})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, delegator, claimableGwei);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);
    });

    it('5. Substake executes multiple times in a row', async () => {
        expect(await validator.totalStake()).eq(currTotalStakeGwei);
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(0);

        let delta = utils.ethToGwei(100000); // 100000000000000 wei
        await expect(validator.subStake(delta)).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, adminAddr, currTotalStakeGwei.sub(delta));

        expect(await validator.testGetClaimableUnbound(vaddr)).eq(delta);

        await expect(validator.subStake(delta)).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, adminAddr, currTotalStakeGwei.sub(delta.mul(2)));

        expect(await validator.testGetClaimableUnbound(vaddr)).eq(delta.mul(2));
        await expect(validator.subStake(delta)).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, adminAddr, currTotalStakeGwei.sub(delta.mul(3)));
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(delta.mul(3));

        // and then settle rewards , set rewards to 2 * 900k
        let currTotalRewards = currTotalStakeGwei.sub(delta).mul(2);
        // validator commission: 50% ==> 900k
        // validator rewards 4/9 ==> 400k
        // delegator rewards 5/9 ==> 500k
        let valExpectRewards = currTotalRewards.div(18).mul(13);
        let delegatorExpectRewards = currTotalRewards.sub(valExpectRewards);

        expect(await validator.testGetClaimableUnbound(vaddr)).eq(delta.mul(3));
        expect(await validator.anyClaimable(currTotalRewards, adminAddr)).eq(valExpectRewards.add(utils.gweiToWei(delta.mul(3))));
        await expect(validator.validatorClaimAny(adminAddr, {value: currTotalRewards})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, adminAddr, valExpectRewards);
        // the delegator has half currTotalRewards as staking rewards
        expect(await validator.anyClaimable(0, delegator)).eq(delegatorExpectRewards);
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(0);
    });

    it('6. SubDelegation executes multiple times in a row', async () => {
        let delta = utils.ethToGwei(50000);
        // currTotalRewards 2m
        let settledRewards = currTotalStakeGwei.mul(2);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta);

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta.mul(2)));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta.mul(2));

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta.mul(3)));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta.mul(3));

        let accRewardsPerStake = await validator.accRewardsPerStake();
        let dlg = await validator.delegators(delegator);
        let claimableGwei = accRewardsPerStake.mul(dlg.stakeGWei).add(dlg.settled).sub(dlg.debt);
        await expect(validator.delegatorClaimAny(delegator, {value: 0})).to
            .emit(validator, "RewardsWithdrawn")
            .withArgs(vaddr, delegator, claimableGwei);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);
    });
})

describe("Validator pushing test", function () {
    let signers
    let owner
    let factory
    let validator // contract
    let vSigner; // validator
    let vaddr; // validator address
    let delegator // address
    let adminSigner; // admin signer
    let adminAddr; // admin address

    let commissionRate = 50;
    let currTotalStakeGwei;
    let stake = utils.ethToGwei(500000);

    beforeEach(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        adminSigner = signers[2];
        adminAddr = adminSigner.address;
        delegator = signers[3].address;

        factory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);
        validator = await factory.deploy(vaddr, adminAddr, commissionRate, stake, true, State.Idle);
        await validator.addDelegation(stake, delegator);
        currTotalStakeGwei = stake.mul(2);
    });

    it('1. punish with correct rewards calculation', async () => {
        let oldTotalStake = await validator.totalStake();
        let sendRewardsGwei = currTotalStakeGwei.mul(2);
        let oldAccRewardsPerStake = await validator.accRewardsPerStake();
        let receivedRewards = TestHandleReceivedRewards(sendRewardsGwei, oldAccRewardsPerStake, commissionRate, oldTotalStake)
        let accRewardsPerStake = receivedRewards.accRewardsPerStake;
        let currCommission = receivedRewards.currCommission;
        let oldTotalUnWithdrawn = await validator.totalUnWithdrawn();
        let oldSelfStakeGWei = await validator.selfStakeGWei();
        let oldPendingUnbound = await validator.testGetClaimableUnbound(vaddr);
        let oldSelfUnWithdrawn = oldSelfStakeGWei.add(oldPendingUnbound);
        let oldAccPunishFactor = await validator.accPunishFactor();
        //console.log(await utils.getLatestCoinbase());
        await expect(validator.punish(params.EvilPunishFactor, {value: sendRewardsGwei})).to
            .emit(validator, "StateChanged")
            .withArgs(vaddr, "0xC014BA5EC014ba5ec014Ba5EC014ba5Ec014bA5E", State.Idle, State.Jail);

        let slashAmount = oldTotalUnWithdrawn.mul(params.EvilPunishFactor).div(params.PunishBase);
        let newTotalUnWithdrawn = await validator.totalUnWithdrawn();
        expect(newTotalUnWithdrawn).eq(oldTotalUnWithdrawn.sub(slashAmount));

        let selfSlashAmount = oldSelfUnWithdrawn.mul(params.EvilPunishFactor).div(params.PunishBase);
        let newSelfStakeGWei = 0;
        let newPendingUnbound = 0;
        if (oldSelfStakeGWei >= selfSlashAmount) {
            newSelfStakeGWei = oldSelfStakeGWei - selfSlashAmount;
        } else {
            let debt = selfSlashAmount - oldSelfStakeGWei;
            if (newPendingUnbound >= debt) {
                newPendingUnbound = oldPendingUnbound - debt;
            } else {
                newPendingUnbound = 0;
            }
            newSelfStakeGWei = 0;
        }
        expect(await validator.testGetClaimableUnbound(vaddr)).eq(newPendingUnbound);
        expect(await validator.selfStakeGWei()).eq(newSelfStakeGWei);
        expect(await validator.accPunishFactor()).eq(oldAccPunishFactor.add(params.EvilPunishFactor));
        expect(await validator.accRewardsPerStake()).eq(accRewardsPerStake);
        expect(await validator.currCommission()).eq(currCommission);
    });

    it('2. calcDelegatorPunishment with correct rewards calculation', async () => {
        let accPunishFactor = await validator.accPunishFactor();
        let dlg = await validator.delegators(delegator);
        expect(dlg.punishFree).eq(0);
        let oldPendingUnbound = await validator.testGetClaimableUnbound(vaddr)
        let deltaFactor = accPunishFactor.sub(dlg.punishFree);
        let totalDelegation = dlg.stakeGWei.add(oldPendingUnbound);
        let amount = totalDelegation.mul(deltaFactor).div(params.PunishBase);
        expect(await validator.testCalcDelegatorPunishment(delegator)).eq(amount);
    });

    it('3. Create test data for addunboundrecord in advance', async () => {
        let delta = utils.ethToGwei(50000);
        // currTotalRewards 2m
        let settledRewards = currTotalStakeGwei.mul(2);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta);

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta.mul(2)));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta.mul(2));

        await expect(validator.subDelegation(delta, delegator, {value: settledRewards})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, currTotalStakeGwei.sub(delta.mul(3)));
        expect(await validator.testGetClaimableUnbound(delegator)).eq(delta.mul(3));

        let dlg = await validator.delegators(delegator);
        let oldStakeGWei = dlg.stakeGWei;
        let oldTotalStake = await validator.totalStake();
        let sendRewardsGwei = currTotalStakeGwei.mul(2);

        await expect(validator.exitDelegation(delegator, {value: sendRewardsGwei})).to
            .emit(validator, "StakesChanged")
            .withArgs(vaddr, delegator, oldTotalStake.sub(oldStakeGWei));

        expect(await validator.testGetClaimableUnbound(delegator)).eq(oldStakeGWei.add(delta.mul(3)));
    });

    it('4. test slashFromUnbound whether there is data overflow', async () => {
        let amountUnbound = await validator.testGetClaimableUnbound(delegator);
        let amountUnboundDiv5 = amountUnbound.div(5);
        for (let i = 1; i <= 5; i ++) {
            await validator.testSlashFromUnbound(delegator, amountUnboundDiv5);
            expect(await validator.testGetClaimableUnbound(delegator)).eq(amountUnbound.sub(amountUnboundDiv5.mul(i)));
        }
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);
        await validator.testSlashFromUnbound(delegator, amountUnboundDiv5);
        expect(await validator.testGetClaimableUnbound(delegator)).eq(0);
        // Old data is deleted correctly
        let newUnbound =  await validator.unboundRecords(delegator);
        expect(newUnbound.count).eq(0);
        expect(newUnbound.startIdx).eq(0);
        expect(newUnbound.pendingAmount).eq(0);
    });
})

function TestHandleReceivedRewards(sendRewards, oldAccRewardsPerStake, commissionRate, oldTotalStake) {
    let c = sendRewards.mul(commissionRate).div(100);
    let newRewards = sendRewards - c;
    let rps = newRewards / oldTotalStake;
    let currCommission = sendRewards- (rps * oldTotalStake);
    let accRewardsPerStake = oldAccRewardsPerStake.add(rps)
    return {accRewardsPerStake, currCommission};
}