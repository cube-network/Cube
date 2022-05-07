const hre = require("hardhat");
const {BigNumber} = require("ethers");
const {expect} = require("chai");
const ethers = hre.ethers;
const utils = require("./utils");

describe("BonusPool test", function(){
    let bonusPool;
    let owner;
    let signers;
    let amount = BigNumber.from(100);

    before(async function () {
        // runs once before the first test in this block
        signers = await hre.ethers.getSigners();
        owner = signers[0];
        let Factory = await hre.ethers.getContractFactory("BonusPool",owner);
        bonusPool = await Factory.deploy();
        await bonusPool.initialize(owner.address, {value: ethers.utils.parseUnits("1000")});
        expect(await bonusPool.bonusEnded()).to.be.eq(false);
        expect(await bonusPool.owner()).to.be.eq(owner.address);
    });

    it('should bindingStake success', async () => {
        for (let i = 1; i < 6; i++) {
            await expect(bonusPool.bindingStake(signers[i].address, amount)).to.emit(bonusPool, "BonusRecordUpdated");
        }
    });
    it('should have no bonus when staking time less then 90 days', async () => {
        // add 89 days
        // let t1 = BigNumber.from( await utils.getLatestTimestamp());
        await hre.network.provider.send("evm_increaseTime", [3600 * 24 * 89]);
        await expect(bonusPool.unbindStakeAndGetBonus(signers[1].address, signers[1].address, amount)).to.emit(bonusPool, "NoBonusOnUnbind").withArgs(signers[1].address, amount, 89);
        // let t2 = BigNumber.from( await utils.getLatestTimestamp());
        // console.log(t2.sub(t1).toNumber())
    });
    it('should update weightedStartTime correctly', async () => {
        // TODO：
    });
})
