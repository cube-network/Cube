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
    FounderLock: 3600,
    releasePeriod: 60,
    releaseCount: 24,

    totalRewards: utils.ethToWei("25000000"),
    rewardsPerBlock: utils.ethToWei("10"),
    epoch: 2,
    ruEpoch: 5,

    singleValStake: utils.ethToWei("2000000"),
    singleValStakeEth: 2000000,
}

describe("Staking test", function () {
    let signers
    let owner
    let factory
    let staking //  contract

    let commissionRate = 50;
    let currTotalStakeGwei = BigNumber.from(0);

    let bonusPool;
    let communityPool;
    let valFactory;

    before(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        vSigner = signers[1];
        vaddr = vSigner.address;
        factory = await hre.ethers.getContractFactory("Staking", owner);
        // let stake = utils.ethToGwei(params.MinSelfStakes);
        // currTotalStakeGwei = stake;
        staking = await factory.deploy();
        expect(staking.address).to.be.properAddress

        // bonusPool and community pool
        let bpFactory = await hre.ethers.getContractFactory("BonusPool", owner);
        let cpFactory = await hre.ethers.getContractFactory("CommunityPool", owner);
        bonusPool = await bpFactory.deploy();
        communityPool = await cpFactory.deploy();

        await bonusPool.initialize(staking.address, {value: params.totalRewards});
        await communityPool.initialize(owner.address);
        expect(await bonusPool.owner()).to.eq(staking.address);
        expect(await communityPool.admin()).to.eq(owner.address);
        expect(await ethers.provider.getBalance(bonusPool.address)).to.eq(params.totalRewards);

        valFactory = await hre.ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator", owner);

    });

    it('1. initialize', async () => {
        let balance = params.singleValStake.mul(3);
        balance = balance.add(params.totalRewards);
        // console.log(utils.weiToEth(balance))
        await staking.initialize(owner.address, params.FounderLock, params.releasePeriod, params.releaseCount,
            params.totalRewards, params.rewardsPerBlock, params.epoch, params.ruEpoch,
            communityPool.address, bonusPool.address, {value: balance});

        expect(await staking.admin()).to.eq(owner.address);
        expect(await ethers.provider.getBalance(staking.address)).to.eq(balance);
    });

    it('2. initValidator', async () => {
        // let balance = await ethers.provider.getBalance(staking.address);
        // console.log(utils.weiToEth(balance));

        for (let i = 1; i < 4; i++) {
            let val = signers[i].address;
            let tx = await staking.initValidator(val, val, 50, params.singleValStakeEth, true);
            let receipt = await tx.wait();
            expect(receipt.status).equal(1);
            currTotalStakeGwei = currTotalStakeGwei.add(utils.ethToGwei(params.singleValStakeEth))
            // console.log(currTotalStakeGwei);
        }

        await expect(staking.initValidator(signers[1].address, signers[1].address, 50, params.singleValStakeEth, true)).to.be.revertedWith("E07");
        await expect(staking.initValidator(signers[4].address, signers[4].address, 50, params.singleValStakeEth, true)).to.be.revertedWith("E15");
        console.log(await ethers.provider.getBlockNumber());
    });

    it('3. check Validator contract', async () => {
        for (let i = 0; i < 3; i++) {
            let valAddress = signers[i + 1].address;
            let valContractAddr = await staking.valMaps(valAddress);
            let val = valFactory.attach(valContractAddr);
            expect(await val.owner()).to.eq(staking.address);
            expect(await val.validator()).to.eq(valAddress);
            expect(await val.manager()).to.eq(valAddress);
            expect(await val.selfStakeGWei()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.totalStake()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.totalUnWithdrawn()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.state()).to.eq(State.Ready);
        }
    });
    it('4. calc rewards', async () => {
        // update accRewardsPerStake by updateRewardsInfo
        while (true) {
            let number = await ethers.provider.getBlockNumber();
            if ((number + 1) % params.ruEpoch !== 0) {
                await utils.mineEmptyBlock();
            } else {
                break;
            }
        }
        let tx = await staking.updateRewardsInfo(params.rewardsPerBlock);
        let receipt = await tx.wait();
        expect(receipt.status).to.eq(1);
        let number = await ethers.provider.getBlockNumber();
        // console.log("blockNumber: ", number)
        let stakeGwei = utils.ethToGwei(params.singleValStakeEth);
        // let totalStakeGwei = stakeGwei.mul(3);
        let expectAccRPS = params.rewardsPerBlock.mul(number);
        expectAccRPS =  expectAccRPS.div(BigNumber.from(currTotalStakeGwei));

        console.log(expectAccRPS)
        // validator claimable
        let claimable = expectAccRPS.mul(stakeGwei);
        expect(await staking.anyClaimable(signers[1].address,signers[1].address)).to.eq(claimable);
        // console.log("blockNumber: ", await ethers.provider.getBlockNumber())

        // claim any
        // when sending a transaction, there will be a new block, so the rewards increase
        // Notice: how many times to calculate and when to calculate, should be exactly the same in the contract,
        // so to avoids the inaccurate integer calculation. For example: 300/3 == 100, but 100/3 + 100/3 + 100/3 == 99
        expectAccRPS = params.rewardsPerBlock.mul(number + 1)
        expectAccRPS =  expectAccRPS.div(BigNumber.from(currTotalStakeGwei));
        console.log(expectAccRPS)
        let valContractAddr = await staking.valMaps(signers[1].address);
        let val = valFactory.attach(valContractAddr);

        let staking2 = staking.connect(signers[1]);
        claimable = expectAccRPS.mul(stakeGwei);
        tx = await staking2.validatorClaimAny(signers[1].address);
        console.log("accRewardsPerStake ", await staking2.accRewardsPerStake());
        await expect(tx).to
            .emit(val, "RewardsWithdrawn")
            .withArgs(signers[1].address,signers[1].address, claimable);
        await expect(tx).to
            .emit(staking,"ClaimWithoutUnboundStake")
            .withArgs(signers[1].address)
    });

})