// Authorized by zero@fairyproof

const { expect, assert } = require("chai");
const { ethers } = require("hardhat");

const State = {
    Idle: 0,
    Ready: 1,
    Jail: 2,
    Exit: 3
}

function convertNum(num) {
    let big = ethers.BigNumber.from("" + num)
    let str = big.toHexString()
    let index = 0
    for(let i=2;i<str.length;i++) {
        if(str[i] !== "0") {
            index = i;
            break;
        }
    }
    if(index === 0) {
        return str;
    }else {
        return str.substring(0,2) + str.substring(index)
    }
}

const params = {
    MaxStakes: 24000000,
    OverMaxStakes: 24000001,
    ThresholdStakes: 50000,
    MinSelfStakes: 50000,
    StakeUnit: 1,
    FounderLock: 3600,
    releasePeriod: 60,
    releaseCount: 24,

    totalRewards: ethers.utils.parseEther("25000000"),
    rewardsPerBlock: ethers.utils.parseEther("10"),
    epoch: 200,
    ruEpoch: 5,

    singleValStake: ethers.utils.parseEther("500000"),
    singleValStakeEth: 500000,
}

describe("Staking Test", function () {
    let instance;
    let owner,user1,user2,users;
    let valFactory;
    let bonus;
    let communityPool;
 

    beforeEach( async function() {
        let Staking = await ethers.getContractFactory("Staking");
        instance = await Staking.deploy();
        [owner,user1,user2,...users] = await ethers.getSigners();
        let BonusPool = await ethers.getContractFactory("BonusPool");
        bonus = await BonusPool.deploy();
        await bonus.initialize(instance.address);
        let CommunityPool = await ethers.getContractFactory("CommunityPool");
        communityPool = await CommunityPool.deploy();
        await communityPool.initialize(instance.address);
        valFactory = await ethers.getContractFactory("cache/solpp-generated-contracts/Validator.sol:Validator");
        let args = [
            owner.address,params.FounderLock,params.releasePeriod,params.releaseCount,
            params.totalRewards, params.rewardsPerBlock, params.epoch, params.ruEpoch,
            communityPool.address,bonus.address
        ]
        let balance = params.singleValStake.mul(3);
        balance = balance.add(params.totalRewards);
        await instance.initialize(...args,{
            value:balance
        });

    })


    describe("claim test", () => {
        // let val;
        let value = ethers.utils.parseUnits("" + params.singleValStakeEth,"ether");
        beforeEach(async () => {
            for(let i=0;i<3;i++) {
                let _val = users[i].address;
                await instance.initValidator(_val, user1.address, 10, params.singleValStakeEth, true);
            }
        });
        

        it("validatorClaimAny only manager", async () => {
            // get validator contract
            let valContractAddr = await instance.valMaps(users[0].address);
            let validator = valFactory.attach(valContractAddr);
            // check init state
            expect(await validator.state()).to.be.equal(1);

            // update block
            let basicLockEnd = await instance.getBasicLockEnd();
            basicLockEnd = + basicLockEnd.toString();
            let period = params.releaseCount * params.releasePeriod
            await ethers.provider.send("evm_mine",[basicLockEnd + period])

            let bal_init = await ethers.provider.getBalance(validator.address);
            expect(bal_init).to.be.equal(0);

            // add stake 
            await instance.connect(user1).addStake(users[0].address,{
                value:value.mul(2)
            });
            // wait 16  blocks
            await ethers.provider.send("hardhat_mine",["0x10"]);
            //exit stake
            await instance.connect(user1).exitStaking(users[0].address);
            expect(await validator.state()).to.be.equal(3);
            await ethers.provider.send("hardhat_mine",["0x10"]);
            //claim
            await instance.connect(user1).validatorClaimAny(users[0].address);
            expect(await ethers.provider.getBalance(validator.address)).to.be.equal(0);
        });
 

        it("validatorClaimAny mixed delegator and manager", async () => {
            let valContractAddr = await instance.valMaps(users[0].address);
            let validator = valFactory.attach(valContractAddr);
            // update block
            let basicLockEnd = await instance.getBasicLockEnd();
            basicLockEnd = + basicLockEnd.toString();
            let period = params.releaseCount * params.releasePeriod
            await ethers.provider.send("evm_mine",[basicLockEnd + period])
            let bal_init = await ethers.provider.getBalance(validator.address);
            expect(bal_init).to.be.equal(0);
            // add stake 
            await instance.connect(user1).addStake(users[0].address,{
                value:value.mul(2)
            });
            // wait 16  blocks
            await ethers.provider.send("hardhat_mine",["0x10"]);
            // add stake delegate
            await instance.addDelegation(users[0].address,{
                value:ethers.utils.parseEther("1")
            });
            
            await ethers.provider.send("hardhat_mine",["0x10"]);
            //exit stake
            await instance.connect(user1).exitStaking(users[0].address);
            
            //claim should be success
            await ethers.provider.send("hardhat_mine",["0x10"]);
            await instance.connect(user1).validatorClaimAny(users[0].address);
        });
    });
});