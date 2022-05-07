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
    MaxValidators: 21,

    MaxStakes: 24000000,
    OverMaxStakes: 24000001,
    ThresholdStakes: 2000000,
    MinSelfStakes: 150000,
    StakeUnit: 1,
    FounderLock: 3600,
    releasePeriod: 60,
    releaseCount: 100,

    totalRewards: utils.ethToWei("25000000"),
    rewardsPerBlock: utils.ethToWei("10"),
    epoch: 2,
    ruEpoch: 5,

    singleValStake: utils.ethToWei("2000000"),
    singleValStakeEth: 2000000,

    ValidatorFeePercent: 80,
    LazyPunishThreshold: 3,
    DecreaseRate: 1,

    LazyPunishFactor: 1,
    EvilPunishFactor: 10,
    PunishBase: 1000,
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
        let balance = params.singleValStake.mul(24);
        balance = balance.add(params.totalRewards);
        // console.log(utils.weiToEth(balance))
        await staking.initialize(owner.address, params.FounderLock, params.releasePeriod, params.releaseCount,
            params.totalRewards, params.rewardsPerBlock, params.epoch, params.ruEpoch,
            communityPool.address, bonusPool.address, {value: balance});

        expect(await staking.admin()).to.eq(owner.address);
        let timestamp = await utils.getLatestTimestamp();
        expect(await staking.getBasicLockEnd()).to.eq(parseInt(timestamp, 16) + params.FounderLock);
        expect(await staking.getReleasePeriod()).to.eq(params.releasePeriod);
        expect(await staking.getReleaseCount()).to.eq(params.releaseCount);
        expect(await staking.getTotalStakingRewards()).to.eq(params.totalRewards);
        expect(await staking.currRewardsPerBlock()).to.eq(params.rewardsPerBlock);
        expect(await staking.blockEpoch()).to.eq(params.epoch);
        expect(await staking.rewardsUpdateEpoch()).to.eq(params.ruEpoch);
        expect(await staking.bonusPool()).to.eq(bonusPool.address);
        expect(await staking.communityPool()).to.eq(communityPool.address);
        expect(await ethers.provider.getBalance(staking.address)).to.eq(balance);
    });

    it('2. initValidator', async () => {
        // let balance = await ethers.provider.getBalance(staking.address);
        // console.log(utils.weiToEth(balance));

        for (let i = 1; i < 25; i++) {
            let val = signers[i].address;
            let admin = signers[25 + i].address;
            let tx = await staking.initValidator(val, admin, 50, params.singleValStakeEth, true);
            let receipt = await tx.wait();
            expect(receipt.status).equal(1);
            currTotalStakeGwei = currTotalStakeGwei.add(utils.ethToGwei(params.singleValStakeEth))
        }
        expect(await staking.totalStakeGWei()).to.eq(currTotalStakeGwei);

        for (let i = 1; i < 4; i++) {
            let addr = await staking.allValidatorAddrs(i - 1);
            expect(signers[i].address).to.eq(addr);
            expect(await staking.valMaps(addr)).to.be.properAddress;
            let info = await staking.valInfos(addr);
            expect(info.stakeGWei).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(info.debt).to.eq(0);
            expect(info.unWithdrawn).to.eq(utils.ethToGwei(params.singleValStakeEth));

            let founderLock = await staking.founders(addr);
            expect(founderLock.initialStakeGWei).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(founderLock.unboundStakeGWei).to.eq(0);
            expect(founderLock.locking).to.eq(true);
        }

        await expect(staking.initValidator(signers[1].address, signers[1].address, 50, params.singleValStakeEth, true)).to.be.revertedWith("E07");
        await expect(staking.initValidator(signers[25].address, signers[25].address, 50, params.singleValStakeEth, true)).to.be.revertedWith("E15");
        //console.log(await ethers.provider.getBlockNumber());
    });

    it('3. check removePermission', async () => {
        expect(await staking.isOpened()).to.eq(false);
        await expect(staking.removePermission()).to
            .emit(staking, "PermissionLess")
            .withArgs(true);
        expect(await staking.isOpened()).to.eq(true);
        await expect(staking.removePermission()).to.be.revertedWith("E16");
    });

    it('4. check getTopValidators', async () => {
        let topValidators = await staking.getTopValidators(0);
        expect(topValidators.length).to.eq(params.MaxValidators);
        topValidators = await staking.getTopValidators(10);
        expect(topValidators.length).to.eq(10);
        topValidators = await staking.getTopValidators(24);
        expect(topValidators.length).to.eq(24);
        topValidators = await staking.getTopValidators(100);
        expect(topValidators.length).to.eq(24);
    });

    it('5. check Validator contract', async () => {
        for (let i = 1; i < 25; i++) {
            let valAddress = signers[i].address;
            let adminAddress = signers[25 + i].address;
            let valContractAddr = await staking.valMaps(valAddress);
            let val = valFactory.attach(valContractAddr);
            expect(await val.owner()).to.eq(staking.address);
            expect(await val.validator()).to.eq(valAddress);
            expect(await val.manager()).to.eq(adminAddress);
            expect(await val.selfStakeGWei()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.totalStake()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.totalUnWithdrawn()).to.eq(utils.ethToGwei(params.singleValStakeEth));
            expect(await val.state()).to.eq(State.Ready);
        }
    });

    it('6. check updateActiveValidatorSet', async () => {
        let activeValidators = await staking.getActiveValidators();
        expect(activeValidators.length).to.eq(0);
        let topValidators = await staking.getTopValidators(0);
        expect(topValidators.length).to.eq(params.MaxValidators);
        while (true) {
            let number = await ethers.provider.getBlockNumber();
            if ((number + 1) % params.epoch !== 0) {
                await utils.mineEmptyBlock();
            } else {
                break;
            }
        }

        await staking.updateActiveValidatorSet(topValidators);
        activeValidators = await staking.getActiveValidators();
        expect(activeValidators.length).to.eq(params.MaxValidators);
    });

    it('7. calc rewards', async () => {
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

        //console.log(expectAccRPS)
        // validator claimable
        let claimable = expectAccRPS.mul(stakeGwei);
        expect(await staking.anyClaimable(signers[1].address,signers[1 + 25].address)).to.eq(claimable);
        // console.log("blockNumber: ", await ethers.provider.getBlockNumber())

        // claim any
        // when sending a transaction, there will be a new block, so the rewards increase
        // Notice: how many times to calculate and when to calculate, should be exactly the same in the contract,
        // so to avoids the inaccurate integer calculation. For example: 300/3 == 100, but 100/3 + 100/3 + 100/3 == 99
        expectAccRPS = params.rewardsPerBlock.mul(number + 1)
        expectAccRPS =  expectAccRPS.div(BigNumber.from(currTotalStakeGwei));
        //console.log(expectAccRPS)
        let valContractAddr = await staking.valMaps(signers[1].address);
        let val = valFactory.attach(valContractAddr);

        let staking2 = staking.connect(signers[1 + 25]);
        claimable = expectAccRPS.mul(stakeGwei);
        tx = await staking2.validatorClaimAny(signers[1].address);
        //console.log("accRewardsPerStake ", await staking2.accRewardsPerStake());
        await expect(tx).to
            .emit(val, "RewardsWithdrawn")
            .withArgs(signers[1].address,signers[1 + 25].address, claimable);
        await expect(tx).to
            .emit(staking,"ClaimWithoutUnboundStake")
            .withArgs(signers[1].address)
    });

    it('8. check distributeBlockFee', async () => {
        let activeValidators = await staking.getActiveValidators();
        let cnt = activeValidators.length;
        let balances = [];
        for (let i = 0; i < cnt; i++) {
            let val = await staking.valMaps(activeValidators[i]);
            balances[i] = await ethers.provider.getBalance(val);
        }
        let communityPoolbalances = await ethers.provider.getBalance(communityPool.address);

        let stake = utils.ethToGwei(100);
        let blockFee = stake.mul(cnt);

        let tx = await staking.distributeBlockFee({value: blockFee});
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        let feePerValidator = blockFee.mul(params.ValidatorFeePercent).div(100).div(cnt)

        for (let i = 0; i < activeValidators.length; i++) {
            let val = await staking.valMaps(activeValidators[i]);
            let balance = await ethers.provider.getBalance(val);
            expect(balance.sub(balances[i])).equal(feePerValidator);
        }
        let newCommunityPoolbalances = communityPoolbalances.add(blockFee.mul(100 - params.ValidatorFeePercent).div(100));
        expect(await ethers.provider.getBalance(communityPool.address)).equal(newCommunityPoolbalances);
    });

    it('9. check lazyPunish', async () => {
        let activeValidators = await staking.getActiveValidators();
        let cnt = activeValidators.length;

        for (let i = 0; i < cnt; i++) {
            let tx = await staking.lazyPunish(activeValidators[i]);
            let receipt = await tx.wait();
            expect(receipt.status).equal(1);
        }

        for (let i = 0; i < cnt; i++) {
            let lazyVal = await staking.lazyPunishedValidators(i);
            expect(await staking.getPunishRecord(activeValidators[i])).equal(1);
            expect(lazyVal).equal(activeValidators[i]);
        }
        /*while (true) {
            let number = await ethers.provider.getBlockNumber();
            if ((number + 1) % params.ruEpoch !== 0) {
                await utils.mineEmptyBlock();
            } else {
                break;
            }
        }*/
        let topVals = await staking.getTopValidators(100);
        let oldInfo = await staking.valInfos(activeValidators[0]);
        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let valContractAddr = await staking.valMaps(activeValidators[0]);
        let oldBalance = await ethers.provider.getBalance(valContractAddr);
        let oldAccRewardsPerStake = await staking.simulateUpdateRewardsRecord();
        for (let i = 1; i < params.LazyPunishThreshold; i++) {
            let tx = await staking.lazyPunish(activeValidators[0]);
            let receipt = await tx.wait();
            expect(receipt.status).equal(1);
            if (i < params.LazyPunishThreshold - 1) {
                let missedBlocksCounter = await staking.getPunishRecord(activeValidators[0]);
                expect(missedBlocksCounter).equal(i + 1);
            } else { // doSlash
                // console.log("doSlash")
                // remove from ranking immediately
                expect(await staking.getPunishRecord(activeValidators[0])).equal(0);
                let newTopVals = await staking.getTopValidators(100);
                expect(newTopVals.length).equal(topVals.length - 1);
                for (let i = 0; i < newTopVals.length; i++) {
                    expect(activeValidators[0] !== newTopVals[i]).equal(true);
                }

                let slashAmount = oldInfo.unWithdrawn.mul(params.LazyPunishFactor).div(params.PunishBase);
                let amountFromCurrStakes = slashAmount;
                if (oldInfo.stakeGWei < slashAmount) {
                    amountFromCurrStakes = oldInfo.stakeGWei;
                }
                let newInfo = await staking.valInfos(activeValidators[0]);
                expect(newInfo.stakeGWei).to.eq(oldInfo.stakeGWei.sub(amountFromCurrStakes));
                let accRewardsPerStake = await staking.accRewardsPerStake();
                expect(newInfo.debt).to.eq(accRewardsPerStake.mul(newInfo.stakeGWei));
                expect(newInfo.unWithdrawn).to.eq(oldInfo.unWithdrawn.sub(slashAmount));
                expect(await staking.totalStakeGWei()).to.eq(oldTotalStakeGWei.sub(amountFromCurrStakes));
                // Only the test token has been transferred. The detailed test is checked by the verifier's contract test case
                // let newBalance = await ethers.provider.getBalance(valContractAddr);
                // let settledRewards = oldAccRewardsPerStake.mul(oldInfo.stakeGWei).sub(oldInfo.debt);
                // expect(newBalance).to.eq(oldBalance.add(settledRewards));
                // TODO
            }
        }
    });

    it('10. check doubleSignPunish', async () => {
        let activeValidators = await staking.getActiveValidators();
        let val = activeValidators[1];

        let topVals = await staking.getTopValidators(100);
        let oldInfo = await staking.valInfos(val);
        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let valContractAddr = await staking.valMaps(val);
        let testPunishHash = "0x47e6e9803bc15fb3fd83a29013a2b264dae9df41fe0272d0fa73f35a727c2f55";

        let tx = await staking.doubleSignPunish(testPunishHash, val);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);
        expect(await staking.isDoubleSignPunished(testPunishHash)).equal(true);

        let newTopVals = await staking.getTopValidators(100);
        expect(newTopVals.length).equal(topVals.length - 1);
        for (let i = 0; i < newTopVals.length; i++) {
            expect(val !== newTopVals[i]).equal(true);
        }

        let slashAmount = oldInfo.unWithdrawn.mul(params.EvilPunishFactor).div(params.PunishBase);
        let amountFromCurrStakes = slashAmount;
        if (oldInfo.stakeGWei < slashAmount) {
            amountFromCurrStakes = oldInfo.stakeGWei;
        }
        let newInfo = await staking.valInfos(val);
        expect(newInfo.stakeGWei).to.eq(oldInfo.stakeGWei.sub(amountFromCurrStakes));
        let accRewardsPerStake = await staking.accRewardsPerStake();
        expect(newInfo.debt).to.eq(accRewardsPerStake.mul(newInfo.stakeGWei));
        expect(newInfo.unWithdrawn).to.eq(oldInfo.unWithdrawn.sub(slashAmount));
        expect(await staking.totalStakeGWei()).to.eq(oldTotalStakeGWei.sub(amountFromCurrStakes));

        await expect(staking.doubleSignPunish(testPunishHash, val)).to.be.revertedWith("E06");
    });

    it('11. Multiple crimes during punishment', async () => {
        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let activeValidators = await staking.getActiveValidators();
        let val = activeValidators[1];
        let oldInfo = await staking.valInfos(val);
        for (let i = 0; i < params.LazyPunishThreshold; i++) {
            let tx = await staking.lazyPunish(val);
            let receipt = await tx.wait();
            expect(receipt.status).equal(1);
        }
        let slashAmount = oldInfo.unWithdrawn.mul(params.LazyPunishFactor).div(params.PunishBase);
        let amountFromCurrStakes = slashAmount;
        if (oldInfo.stakeGWei < slashAmount) {
            amountFromCurrStakes = oldInfo.stakeGWei;
        }
        let newInfo = await staking.valInfos(val);
        expect(newInfo.stakeGWei).to.eq(oldInfo.stakeGWei.sub(amountFromCurrStakes));
        let accRewardsPerStake = await staking.accRewardsPerStake();
        expect(newInfo.debt).to.eq(accRewardsPerStake.mul(newInfo.stakeGWei));
        expect(newInfo.unWithdrawn).to.eq(oldInfo.unWithdrawn.sub(slashAmount));
        expect(await staking.totalStakeGWei()).to.eq(oldTotalStakeGWei.sub(amountFromCurrStakes));
    });

    it('12. check registerValidator', async () => {
        let signer = signers[51];
        let admin = signers[52];
        let val = signer.address;
        let valAdmin = admin.address;

        let stakeWei = utils.ethToWei(params.MinSelfStakes);
        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let oldLength = await staking.getAllValidatorsLength();
        let tx = await staking.registerValidator(val, valAdmin, 50, true, {value: stakeWei});
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);
        await expect(tx).to
            .emit(staking, "ValidatorRegistered")
            .withArgs(val, valAdmin, 50, utils.ethToGwei(params.MinSelfStakes), State.Idle);
        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(val, oldTotalStakeGWei, oldTotalStakeGWei.add(utils.ethToGwei(params.MinSelfStakes)))
        let timestamp = await utils.getLatestTimestamp();
        await expect(tx).to
            .emit(bonusPool,"BonusRecordUpdated")
            .withArgs(val, params.MinSelfStakes, parseInt(timestamp, 16))

        let newLength = await staking.getAllValidatorsLength();
        expect(newLength).equal(oldLength.add(1));

        let lastAddVal = await staking.allValidatorAddrs(newLength.sub(1));
        expect(lastAddVal).equal(val);
    });

    it('13. check addStake', async () => {
        let signer = signers[1];
        let admin = signers[25 + 1];
        let val = signer.address;
        let valAdmin = admin.address;


        let stakeWei = utils.ethToWei(params.MinSelfStakes);
        let diffWei = utils.ethToWei(params.ThresholdStakes - params.MinSelfStakes);
        let diffGwei = utils.ethToGwei(params.ThresholdStakes - params.MinSelfStakes);

        let stakingErrorAdmin = staking.connect(signers[2]);
        await expect(stakingErrorAdmin.addStake("0x0000000000000000000000000000000000000000", {value: diffWei})).to.be.revertedWith("E08");
        await expect(stakingErrorAdmin.addStake(val, {value: diffWei})).to.be.revertedWith("E02");

        let stakingLocked = staking.connect(admin);
        await expect(stakingLocked.addStake(val, {value: diffWei})).to.be.revertedWith("E22");

        let signerUnlocked = signers[51];
        let adminUnlocked = signers[52];
        let stakingUnlocked = staking.connect(adminUnlocked);
        let oldTotalStakeGWei = await staking.totalStakeGWei();

        let valContractAddr = await staking.valMaps(signerUnlocked.address);
        let valContract = valFactory.attach(valContractAddr);
        let oldValTotalStake = await valContract.totalStake();

        let tx = await stakingUnlocked.addStake(signerUnlocked.address, {value: diffWei.div(2)});
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);
        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signerUnlocked.address, oldTotalStakeGWei, oldTotalStakeGWei.add(diffGwei.div(2)))
        await expect(tx).to
            .emit(valContract,"StakesChanged")
            .withArgs(signerUnlocked.address, adminUnlocked.address, oldValTotalStake.add(diffGwei.div(2)))

        let delegator = signers[53];
        let stakingDelegator = staking.connect(delegator);

        await expect(stakingErrorAdmin.addDelegation("0x0000000000000000000000000000000000000000", {value: diffWei})).to.be.revertedWith("E08");
        tx = await stakingDelegator.addDelegation(signerUnlocked.address, {value: diffWei.div(2)});
        receipt = await tx.wait();
        expect(receipt.status).equal(1);
        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signerUnlocked.address, oldTotalStakeGWei.add(diffGwei.div(2)), oldTotalStakeGWei.add(diffGwei))

        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signerUnlocked.address, delegator.address, State.Idle, State.Ready)

        await expect(tx).to
            .emit(valContract,"StakesChanged")
            .withArgs(signerUnlocked.address, delegator.address, oldValTotalStake.add(diffGwei))
    });

    it('14. check subStake', async () => {
        // locking == true
        let signer2 = signers[2];
        let admin2 = signers[27];
        // locking == false
        let signer50 = signers[51];
        let admin50 = signers[52];

        let deltaEth = 20000;

        // Do substake when the node is in the locking == true
        let stakingLocked = staking.connect(admin2);
        await expect(stakingLocked.subStake("0x0000000000000000000000000000000000000000", deltaEth)).to.be.revertedWith("E08");
        await expect(stakingLocked.subStake(signer50.address, deltaEth)).to.be.revertedWith("E02");
        await expect(stakingLocked.subStake(signer2.address, deltaEth)).to.be.revertedWith("E22");

        // Calculate the upper limit of substake in advance
        // canRelease = 2000000 / 100
        let forceTimeDiff = params.releasePeriod;
        let tx = await staking.testReduceBasicLockEnd(forceTimeDiff);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        let canReleaseCnt = forceTimeDiff / params.releasePeriod;
        let canReleaseAmount = utils.ethToGwei(params.singleValStakeEth).mul(canReleaseCnt).div(params.releaseCount);
        expect(canReleaseAmount).to.eq(utils.ethToGwei(params.singleValStakeEth).div(params.releaseCount));

        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let valContractAddr = await staking.valMaps(signer2.address);
        let val = valFactory.attach(valContractAddr);
        expect(await val.state()).equal(2); //Jail
        await expect(stakingLocked.subStake(signer2.address, deltaEth + 1)).to.be.revertedWith("E22");

        let signer20 = signers[20];
        let admin20 = signers[45];
        stakingLocked = staking.connect(admin20);
        valContractAddr = await staking.valMaps(signer20.address);
        val = valFactory.attach(valContractAddr);
        expect(await val.state()).equal(1);
        tx = await stakingLocked.subStake(signer20.address, deltaEth);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signer20.address, oldTotalStakeGWei, oldTotalStakeGWei.sub(utils.ethToGwei(deltaEth)))

        // The current released amount has exceeded the unlocked amount
        await expect(stakingLocked.subStake(signer20.address, deltaEth)).to.be.revertedWith("E22");

        // When it expires again, it can release the new amount of tokens
        tx = await staking.testReduceBasicLockEnd(forceTimeDiff * 2);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        tx = await stakingLocked.subStake(signer20.address, deltaEth);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signer20.address, oldTotalStakeGWei.sub(utils.ethToGwei(deltaEth)), oldTotalStakeGWei.sub(utils.ethToGwei(deltaEth).mul(2)))

        // locking == false; Unlimited amount of subStake
        oldTotalStakeGWei = await staking.totalStakeGWei();
        // Do substake when the node is in the locking == false
        let stakingUnLocked = staking.connect(admin50);
        tx = await stakingUnLocked.subStake(signer50.address, deltaEth * 2);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signer50.address, oldTotalStakeGWei, oldTotalStakeGWei.sub(utils.ethToGwei(deltaEth * 2)))
    });

    it('15. check subDelegation', async () => {
        // It will not be restricted because of the locked state of the node
        let delegator = signers[53];
        let stakingDelegator = staking.connect(delegator);
        let signer20 = signers[20];
        let admin20 = signers[45];
        let diffWei = utils.ethToWei(params.ThresholdStakes - params.MinSelfStakes);
        let diffGwei = utils.ethToGwei(params.ThresholdStakes - params.MinSelfStakes);
        let diffEther= params.ThresholdStakes - params.MinSelfStakes;
        let valContractAddr = await staking.valMaps(signer20.address);
        let valContract = valFactory.attach(valContractAddr);
        let oldTotalStakeGWei = await staking.totalStakeGWei();
        let oldValTotalStake = await valContract.totalStake();

        let tx = await stakingDelegator.addDelegation(signer20.address, {value: diffWei.div(2)});
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);
        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signer20.address, oldTotalStakeGWei, oldTotalStakeGWei.add(diffGwei.div(2)))

        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signer20.address, delegator.address, State.Idle, State.Ready)

        await expect(tx).to
            .emit(valContract,"StakesChanged")
            .withArgs(signer20.address, delegator.address, oldValTotalStake.add(diffGwei.div(2)))

        await expect(stakingDelegator.subDelegation("0x0000000000000000000000000000000000000000", diffEther / 2)).to.be.revertedWith("E08");
        await expect(stakingDelegator.subDelegation(signer20.address, diffEther)).to.be.revertedWith("E24");
        await expect(stakingDelegator.subDelegation(signer20.address, 0)).to.be.revertedWith("E23");
        tx = await stakingDelegator.subDelegation(signer20.address, diffEther / 2);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        await expect(tx).to
            .emit(staking,"TotalStakeGWeiChanged")
            .withArgs(signer20.address, oldTotalStakeGWei.add(diffGwei.div(2)), oldTotalStakeGWei)
    });

    it('16. check exitStaking', async () => {
        // locking == true && Jail
        let signer2 = signers[2];
        let admin2 = signers[27];
        // locking == true
        let signer20 = signers[20];
        let admin20 = signers[45];
        // locking == false
        let signer50 = signers[51];
        let admin50 = signers[52];

        let staking2 = staking.connect(admin2);
        await expect(staking2.exitStaking(signer2.address)).to.be.revertedWith("E22");

        let staking20 = staking.connect(admin20);
        await expect(staking20.exitStaking(signer20.address)).to.be.revertedWith("E22");

        /*let staking50 = staking.connect(admin50);
        let tx = await staking50.exitStaking(signer50.address);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        let valContractAddr = await staking.valMaps(signer50.address);
        let valContract = valFactory.attach(valContractAddr);
        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signer50.address, admin50.address, State.Idle, State.Exit)*/

        // Forced arrival at the end of the lock period
        tx = await staking.testReduceBasicLockEnd(params.releasePeriod * params.releaseCount);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        // Jail
        staking2 = staking.connect(admin2);
        tx = await staking2.exitStaking(signer2.address);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        valContractAddr = await staking.valMaps(signer2.address);
        valContract = valFactory.attach(valContractAddr);
        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signer2.address, admin2.address, State.Jail, State.Exit)

        // Idle
        staking20 = staking.connect(admin20);
        tx = await staking20.exitStaking(signer20.address);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        valContractAddr = await staking.valMaps(signer20.address);
        valContract = valFactory.attach(valContractAddr);
        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signer20.address, admin20.address, State.Idle, State.Exit)
    });

    it('17. check exitDelegation', async () => {
        let diffWei = utils.ethToWei(params.ThresholdStakes - params.MinSelfStakes);
        // Jail
        let signer2 = signers[2];
        let admin2 = signers[27];
        // Exit
        let signer20 = signers[20];
        let admin20 = signers[45];
        // Idle
        let signer50 = signers[51];
        let admin50 = signers[52];

        let delegator = signers[53];
        let stakingDelegator = staking.connect(delegator);

        // Add some data in advance
        await expect(stakingDelegator.addDelegation(signer2.address, {value: diffWei.div(2)})).to.be.revertedWith("E28");
        await expect(stakingDelegator.addDelegation(signer20.address, {value: diffWei.div(2)})).to.be.revertedWith("E28");

        tx = await stakingDelegator.addDelegation(signer50.address, {value: diffWei.div(2)});
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        // Jail
        await expect(stakingDelegator.exitDelegation(signer2.address)).to.be.revertedWith("E28");
        // Exit
        await expect(stakingDelegator.exitDelegation(signer20.address)).to.be.revertedWith("E28");

        // Idle
        tx = await stakingDelegator.exitDelegation(signer50.address);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        valContractAddr = await staking.valMaps(signer50.address);
        valContract = valFactory.attach(valContractAddr);
        await expect(tx).to
            .emit(valContract,"StateChanged")
            .withArgs(signer50.address, delegator.address, State.Ready, State.Idle)
    });
})