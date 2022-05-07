// We require the Hardhat Runtime Environment explicitly here. This is optional
// but useful for running the script in a standalone fashion through `node <script>`.
//
// When running the script with `npx hardhat run <script>` you'll find the Hardhat
// Runtime Environment's members available in the global scope.
const { expect } = require("chai");
const { ethers } = require("ethers");

describe("GenesisLock contract uint test",function(){

    let GenesisLock;
    let genesisLock;
    let owner;
    let addr1;
    let addr2;
    let addr3;
    let addr4;
    let addr5;

    beforeEach(async function() {
        GenesisLock = await hre.ethers.getContractFactory("GenesisLock");
        genesisLock = await GenesisLock.deploy();

        // await genesisLock.deployed();
        console.log("genesisLock deployed to:", genesisLock.address);

        [owner, addr1, addr2, addr3, addr4,addr5] = await hre.ethers.getSigners();
        console.log("owner address: ",owner.address);
        console.log("addr1 address: ",addr1.address);
        console.log("addr2 address: ",addr2.address);
        console.log("addr3 address: ",addr3.address);
        console.log("addr4 address: ",addr4.address);
        console.log("addr5 address: ",addr5.address);

    })

    it('should initialize success', async function () {
        // failed to with 0
        await expect(genesisLock.initialize(0)).to.be.revertedWith("invalid periodTime");

        // success
        let tx = await genesisLock.initialize(50);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        // failed when retry
        await expect(genesisLock.initialize(10)).to.be.revertedWith("already initialized");

    });

    // address[] memory userAddress,
    // uint256[] memory typeId,
    // uint256[] memory lockedAmount,
    // uint256[] memory lockedTime,
    // uint256[] memory periodAmount
    it("should init success",async function(){
        let tx = await genesisLock.initialize(50);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        const gasPrice = 5000000000;
        const gasLimit = 42000;

        await owner.sendTransaction({
            gasLimit: gasLimit,
            gasPrice: gasPrice,
            to: genesisLock.address,
            value: ethers.utils.parseEther("6000")
        })

        console.log("send eth success");

        const bal = await owner.getBalance();

        console.log("balance of owner:",bal.toString())

        await genesisLock.init(
            [owner.address,addr1.address,addr2.address,addr3.address,addr4.address,addr5.address],
            [1,2,3,4,5,2],
            [ethers.utils.parseEther('1000'),ethers.utils.parseEther('1000'),ethers.utils.parseEther('1000'),ethers.utils.parseEther('1000'),ethers.utils.parseEther('1000'),ethers.utils.parseEther('1000')],
            [100,200,300,50,80,0],
            [10,8,20,10,20,50]);
        console.log("genesisLock init success");

        await getUserInfo();

        const time = await genesisLock.getBlocktimestamp();
        console.log(time.toString())

        await hre.network.provider.send('evm_increaseTime',[100])


        const tx4 = await genesisLock.connect(owner).add();
        console.log(tx4);
        const time1 = await genesisLock.getBlocktimestamp();
        console.log(time1.toString())

        await getUserInfo();

        const claimableAmount = await genesisLock.getClaimableAmount(addr5.address);
        console.log("addr5 claimableAmount:",claimableAmount.toString())

        const claimablePeriod = await genesisLock.getClaimablePeriod(addr5.address);
        console.log("addr5 calaimable Period:",claimablePeriod.toString())
        const bal5bf = await addr5.getBalance()
        console.log("Addr5bf balance is ",bal5bf.toString())

        const tx5 = await genesisLock.connect(addr5).claim();
        console.log(tx5)

        const bal5 = await addr5.getBalance()
        console.log("Addr5 balance is ",bal5.toString())

        const release = await genesisLock.getClaimablePeriod(addr5.address)
        console.log("addr5 release is:",release.toString())

        const timex = await genesisLock.getBlocktimestamp();
        console.log(timex.toString())

        await hre.network.provider.send('evm_increaseTime',[300])

        await getUserInfo();

        const claimableAmount1 = await genesisLock.getClaimableAmount(addr5.address);
        console.log("addr5 claimableAmount1:",claimableAmount1.toString())

        const claimablePeriod1 = await genesisLock.getClaimablePeriod(addr5.address);
        console.log("addr5 calaimable1 Period:",claimablePeriod1.toString())

        const bal5bf1 = await addr5.getBalance()
        console.log("Addr5bf1 balance is ",bal5bf1.toString())

        const tx6 = await genesisLock.connect(addr5).claim();
        console.log(tx6)

        const bal6 = await addr5.getBalance()
        console.log("Addr5 balance 6 is ",bal6.toString())

        const release1 = await genesisLock.getClaimablePeriod(addr5.address)
        console.log("addr5 release is:",release1.toString())

        await getUserInfo();
    })

    async function getUserInfo(){

        const userinfo = await genesisLock.getUserInfo(owner.address);
        console.log("owner:",owner.address);
        console.log("lockType:",userinfo[0].toString());
        console.log("lockAmount:",userinfo[1].toString());
        console.log("firstLockedTime:",userinfo[2].toString());
        console.log("totalLockCount:",userinfo[3].toString());
        console.log("releasedPeriodCount:",userinfo[4].toString());
        console.log("----------------**************----------------")

        const userinfo1 = await genesisLock.getUserInfo(addr1.address);
        console.log("addr1:",addr1.address);
        console.log("lockType:",userinfo1[0].toString());
        console.log("lockAmount:",userinfo1[1].toString());
        console.log("firstLockedTime:",userinfo1[2].toString());
        console.log("totalLockCount:",userinfo1[3].toString());
        console.log("releasedPeriodCount:",userinfo1[4].toString());
        console.log("----------------**************----------------")

        const userinfo2 = await genesisLock.getUserInfo(addr2.address);
        console.log("addr2:",addr2.address);
        console.log("lockType:",userinfo2[0].toString());
        console.log("lockAmount:",userinfo2[1].toString());
        console.log("firstLockedTime:",userinfo2[2].toString());
        console.log("totalLockCount:",userinfo2[3].toString());
        console.log("releasedPeriodCount:",userinfo2[4].toString());
        console.log("----------------**************----------------")

        const userinfo3 = await genesisLock.getUserInfo(addr3.address);
        console.log("addr3:",addr3.address);
        console.log("lockType:",userinfo3[0].toString());
        console.log("lockAmount:",userinfo3[1].toString());
        console.log("firstLockedTime:",userinfo3[2].toString());
        console.log("totalLockCount:",userinfo3[3].toString());
        console.log("releasedPeriodCount:",userinfo3[4].toString());
        console.log("----------------**************----------------")

        const userinfo4 = await genesisLock.getUserInfo(addr4.address);
        console.log("addr4:",addr4.address);
        console.log("lockType:",userinfo4[0].toString());
        console.log("lockAmount:",userinfo4[1].toString());
        console.log("firstLockedTime:",userinfo4[2].toString());
        console.log("totalLockCount:",userinfo4[3].toString());
        console.log("releasedPeriodCount:",userinfo4[4].toString());
        console.log("----------------**************----------------")

        const userinfo5 = await genesisLock.getUserInfo(addr5.address);
        console.log("addr5:",addr5.address);
        console.log("lockType:",userinfo5[0].toString());
        console.log("lockAmount:",userinfo5[1].toString());
        console.log("firstLockedTime:",userinfo5[2].toString());
        console.log("totalLockCount:",userinfo5[3].toString());
        console.log("releasedPeriodCount:",userinfo5[4].toString());
        console.log("----------------**************----------------")
        console.log("~~~~~~~~~~~~~~~~~~~getinfo end~~~~~~~~~~~~~~~~~~~~~~~")
    }
})
