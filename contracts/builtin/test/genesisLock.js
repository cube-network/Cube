// We require the Hardhat Runtime Environment explicitly here. This is optional
// but useful for running the script in a standalone fashion through `node <script>`.
//
// When running the script with `npx hardhat run <script>` you'll find the Hardhat
// Runtime Environment's members available in the global scope.
const {expect, use} = require("chai");
const {BigNumber} = require("ethers");
const hre = require("hardhat");
const ethers = hre.ethers;
const utils = require("./utils");

describe("GenesisLock contract initialize test", function () {

    let GenesisLockFactory;
    let genesisLock;
    let addr0;
    let addr1;
    let addr2;
    let addr3;
    let addr4;
    let addr5;

    let singleLockAmount = ethers.utils.parseEther('1000');
    let periodTime = 50;

    let userInfo;

    beforeEach(async function () {
        GenesisLockFactory = await hre.ethers.getContractFactory("GenesisLock");
        genesisLock = await GenesisLockFactory.deploy();

        [addr0, addr1, addr2, addr3, addr4, addr5] = await hre.ethers.getSigners();
        userInfo = [
            [addr0.address, addr1.address, addr2.address, addr3.address, addr4.address],
            [1, 2, 3, 4, 5],
            [singleLockAmount, singleLockAmount, singleLockAmount, singleLockAmount, singleLockAmount],
            [0, 100, 200, 300, 50],
            [3, 3, 12, 24, 20]
        ]
    })

    it('should initialize success', async function () {
        // failed to with 0
        await expect(genesisLock.initialize(0)).to.be.revertedWith("invalid periodTime");

        // success
        let tx = await genesisLock.initialize(periodTime);
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
    it("should init success", async function () {
        let tx = await genesisLock.initialize(50);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        const gasPrice = 5000000000;
        const gasLimit = 42000;
        let totalCube = ethers.utils.parseEther("5000")
        await addr0.sendTransaction({
            gasLimit: gasLimit,
            gasPrice: gasPrice,
            to: genesisLock.address,
            value: totalCube
        })
        expect(await ethers.provider.getBalance(genesisLock.address)).to.be.eq(totalCube);

        tx = await genesisLock.init(
            userInfo[0],
            userInfo[1],
            userInfo[2],
            userInfo[3],
            userInfo[4]
        );
        receipt = await tx.wait();
        expect(receipt.status).equal(1);

        await expect(genesisLock.init(
            [addr0.address],
            [1],
            [singleLockAmount],
            [100],
            [10])).to.be.revertedWith("user address already exists");
    })
})

describe("GenesisLock contract uint test of claim and append", function () {

    let GenesisLockFactory;
    let genesisLock;
    let addr0;
    let addr1;
    let addr2;
    let addr3;
    let addr4;
    let addr5;

    let singleLockAmount = ethers.utils.parseEther('1000');
    let periodTime = 50;
    let userInfo;

    beforeEach(async function () {
        GenesisLockFactory = await hre.ethers.getContractFactory("GenesisLock");
        genesisLock = await GenesisLockFactory.deploy();

        [addr0, addr1, addr2, addr3, addr4, addr5] = await hre.ethers.getSigners();
        userInfo = [
            [addr0.address, addr1.address, addr2.address, addr3.address, addr4.address],
            [1, 2, 3, 4, 5],
            [singleLockAmount, singleLockAmount, singleLockAmount, singleLockAmount, singleLockAmount],
            [0, 100, 200, 300, 50],
            [3, 3, 12, 24, 20]
        ]
        // success
        let tx = await genesisLock.initialize(periodTime);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        let totalCube = ethers.utils.parseEther("6000")
        const gasPrice = 5000000000;
        await addr0.sendTransaction({
            gasPrice: gasPrice,
            to: genesisLock.address,
            value: totalCube
        })

        expect(await ethers.provider.getBalance(genesisLock.address)).to.be.eq(totalCube);

        tx = await genesisLock.init(
            userInfo[0],
            userInfo[1],
            userInfo[2],
            userInfo[3],
            userInfo[4]);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);
    })


    // address[] memory userAddress,
    // uint256[] memory typeId,
    // uint256[] memory lockedAmount,
    // uint256[] memory lockedTime,
    // uint256[] memory periodAmount
    it("test release without first-lock-time", async function () {
        genesisLock = genesisLock.connect(addr0);
        let addr = addr0.address;
        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await utils.mineEmptyBlock();

        let result = await genesisLock.getClaimableAmount(addr);
        expect(result.claimableAmt).to.eq(singleLockAmount.div(3));
        expect(result.period).to.eq(BigNumber.from(1));

        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(1), singleLockAmount.div(3));

        await hre.network.provider.send('evm_increaseTime', [3 * periodTime])
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(2), singleLockAmount.sub(singleLockAmount.div(3)));

        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await expect(genesisLock.claim()).to
            .revertedWith("Have no token released")
    })

    it("test release with first-lock-time", async function () {
        genesisLock = genesisLock.connect(addr1);
        let addr = addr1.address;
        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await utils.mineEmptyBlock();

        let result = await genesisLock.getClaimableAmount(addr);
        expect(result.claimableAmt).to.eq(BigNumber.from(0));
        expect(result.period).to.eq(BigNumber.from(0));

        await hre.network.provider.send('evm_increaseTime', [110])
        await utils.mineEmptyBlock();
        result = await genesisLock.getClaimableAmount(addr);
        expect(result.claimableAmt).to.eq(singleLockAmount.div(3));
        expect(result.period).to.eq(BigNumber.from(1));

        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(1), singleLockAmount.div(3));

        await hre.network.provider.send('evm_increaseTime', [3 * periodTime])
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(2), singleLockAmount.sub(singleLockAmount.div(3)));
    })

    it("test claim each period", async function () {
        genesisLock = genesisLock.connect(addr1);
        let addr = addr1.address;

        await hre.network.provider.send('evm_increaseTime', [100 + periodTime])
        await utils.mineEmptyBlock();
        result = await genesisLock.getClaimableAmount(addr);
        expect(result.claimableAmt).to.eq(singleLockAmount.div(3));
        expect(result.period).to.eq(BigNumber.from(1));

        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(1), singleLockAmount.div(3));

        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(1), singleLockAmount.div(3));

        await hre.network.provider.send('evm_increaseTime', [2 * periodTime])
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(1), singleLockAmount.div(3).add(1));
    })

    it('should claim no more then userLockAmount', async function () {
        genesisLock = genesisLock.connect(addr0);
        let addr = addr0.address;
        await hre.network.provider.send('evm_increaseTime', [4 * periodTime])
        await utils.mineEmptyBlock();
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(3), singleLockAmount);

        await hre.network.provider.send('evm_increaseTime', [2 * periodTime])
        await expect(genesisLock.claim()).to
            .revertedWith("Have no token released")
    });

    it('should ok with locking amount that can be divided exactly', async function () {
        genesisLock = genesisLock.connect(addr4);
        let addr = addr4.address;

        await hre.network.provider.send('evm_increaseTime', [50])
        for (let i = 0; i < 20; i++) {
            await hre.network.provider.send('evm_increaseTime', [periodTime])
            await expect(genesisLock.claim()).to
                .emit(genesisLock, "ReleaseClaimed")
                .withArgs(addr, BigNumber.from(1), singleLockAmount.div(20));
        }
        await hre.network.provider.send('evm_increaseTime', [2 * periodTime])
        await expect(genesisLock.claim()).to
            .revertedWith("Have no token released")
    });

    it('should have correct user info', async function () {
        //         uint256 typId,           userAddress
        //         uint256 lockedAount,     typeId
        //         uint256 firstLockTime,   lockedAmount
        //         uint256 totalPeriod,     lockedTime
        //         uint256 alreadyClaimed,  periodAmount
        //         uint256 releases

        let userinfoResult = await genesisLock.getUserInfo(addr0.address);
        checkUserInfo(userinfoResult, 0, 0, 0)

        await hre.network.provider.send('evm_increaseTime', [2 * periodTime])
        await utils.mineEmptyBlock();
        userinfoResult = await genesisLock.getUserInfo(addr0.address);
        checkUserInfo(userinfoResult, 0, 0, 2)
        await expect(genesisLock.connect(addr0).claim()).to
            .emit(genesisLock, "ReleaseClaimed")
        userinfoResult = await genesisLock.getUserInfo(addr0.address);
        checkUserInfo(userinfoResult, 0, 2, 2)
    });

    it('should append correctly', async function () {
        await expect(genesisLock.appendLockRecord(addr5.address, 0, 0, 24)).to
            .revertedWith("too trivial")
        await expect(genesisLock.appendLockRecord(addr5.address, 0, 0, 24, {value: ethers.utils.parseEther('100')})).to
            .revertedWith("too trivial")
        await expect(genesisLock.appendLockRecord("0x0000000000000000000000000000000000000000", 0, 0, 24, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("zero address")
        await expect(genesisLock.appendLockRecord(addr5.address, 0, 0, 24, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("need a type id for human read")
        await expect(genesisLock.appendLockRecord(addr5.address, 1, 3600*24*367, 24, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("firstLockTime violating WhitePaper rules")
        await expect(genesisLock.appendLockRecord(addr5.address, 1, 3600*24*365, 0, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("lockPeriodCnt violating WhitePaper rules")
        await expect(genesisLock.appendLockRecord(addr5.address, 1, 3600*24*365, 50, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("lockPeriodCnt violating WhitePaper rules")
        await expect(genesisLock.appendLockRecord(addr1.address, 1, 3600*24*365, 36, {value: ethers.utils.parseEther('1000')})).to
            .revertedWith("user address already have lock-up")

        await expect(genesisLock.appendLockRecord(addr5.address, 1, 3600*24*365, 36, {value: ethers.utils.parseEther('1000')})).to
            .emit(genesisLock,"LockRecordAppened")
            .withArgs(addr5.address, 1,singleLockAmount,3600*24*365, 36)

    });

    function checkUserInfo(userinfoResult, uIdx, expectClaimed, expectReleases) {
        for (let i = 0; i < 4; i++) {
            expect(userinfoResult[i]).to.be.eq(userInfo[i + 1][uIdx])
        }
        expect(userinfoResult[4]).to.be.eq(expectClaimed)
        expect(userinfoResult[5]).to.be.eq(expectReleases)
    }
})

describe("GenesisLock contract uint test of change rights", function () {

    let GenesisLockFactory;
    let genesisLock;
    let addr0;
    let addr1;
    let addr2;
    let addr3;
    let addr4;
    let addr5;

    let singleLockAmount = ethers.utils.parseEther('1000');
    let periodTime = 50;
    let userInfo;

    beforeEach(async function () {
        GenesisLockFactory = await hre.ethers.getContractFactory("GenesisLock");
        genesisLock = await GenesisLockFactory.deploy();

        [addr0, addr1, addr2, addr3, addr4, addr5] = await hre.ethers.getSigners();
        userInfo = [
            [addr0.address, addr1.address, addr2.address],
            [1, 2, 3],
            [singleLockAmount, singleLockAmount, singleLockAmount],
            [0, 100, 200],
            [3, 3, 12]
        ]
        // success
        let tx = await genesisLock.initialize(periodTime);
        let receipt = await tx.wait();
        expect(receipt.status).equal(1);

        let totalCube = ethers.utils.parseEther("3000")
        const gasPrice = 5000000000;
        await addr0.sendTransaction({
            gasPrice: gasPrice,
            to: genesisLock.address,
            value: totalCube
        })

        expect(await ethers.provider.getBalance(genesisLock.address)).to.be.eq(totalCube);

        tx = await genesisLock.init(
            userInfo[0],
            userInfo[1],
            userInfo[2],
            userInfo[3],
            userInfo[4]);
        receipt = await tx.wait();
        expect(receipt.status).equal(1);
    })


    it("should change and accept rights correctly", async function () {
        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr3.address)
        await expect(genesisLock.connect(addr3).acceptAllRights(addr0.address)).to
            .emit(genesisLock, "RightsAccepted")
            .withArgs(addr0.address, addr3.address)

        await rightsCleared(addr0.address)

        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await utils.mineEmptyBlock();
        // new owner can claim normally
        await expect(genesisLock.connect(addr3).claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr3.address, BigNumber.from(1), singleLockAmount.div(3));
    })


    it("should be able to change more times", async function () {
        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr3.address)
        await expect(genesisLock.connect(addr3).acceptAllRights(addr0.address)).to
            .emit(genesisLock, "RightsAccepted")
            .withArgs(addr0.address, addr3.address)

        await rightsCleared(addr0.address)

        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await utils.mineEmptyBlock();
        // new owner can claim normally
        await expect(genesisLock.connect(addr3).claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr3.address, BigNumber.from(1), singleLockAmount.div(3));

        await expect(genesisLock.connect(addr3).changeAllRights(addr4.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr3.address, addr4.address)
        await expect(genesisLock.connect(addr4).acceptAllRights(addr3.address)).to
            .emit(genesisLock, "RightsAccepted")
            .withArgs(addr3.address, addr4.address)

        await rightsCleared(addr3.address)

        await hre.network.provider.send('evm_increaseTime', [periodTime])
        await utils.mineEmptyBlock();
        // new owner can claim normally
        await expect(genesisLock.connect(addr4).claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr4.address, BigNumber.from(1), singleLockAmount.div(3));
    })

    it('should not be changed to an address already has lock-up', async function () {
        await expect(genesisLock.connect(addr0).changeAllRights(addr1.address)).to
            .revertedWith("_to address already have lock-up")
    });

    it('should not be changed from an address without lock-up', async function () {
        await expect(genesisLock.connect(addr3).changeAllRights(addr4.address)).to
            .revertedWith("sender have no lock-up")
    });

    it('should not accept before change or with incorrect from', async function () {
        await expect(genesisLock.connect(addr3).acceptAllRights(addr0.address)).to
            .revertedWith("no changing record")

        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr3.address)

        await expect(genesisLock.connect(addr4).acceptAllRights(addr0.address)).to
            .revertedWith("no changing record")
    });

    it('should only accept one lock-up rights', async function () {
        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr3.address)
        await expect(genesisLock.connect(addr1).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr1.address, addr3.address)

        await expect(genesisLock.connect(addr3).acceptAllRights(addr0.address)).to
            .emit(genesisLock, "RightsAccepted")
            .withArgs(addr0.address, addr3.address)

        await expect(genesisLock.connect(addr3).acceptAllRights(addr1.address)).to
            .revertedWith("sender already have lock-up")
    });

    it('changing operation can be overwrite', async function () {
        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr3.address)
        await expect(genesisLock.connect(addr0).changeAllRights(addr4.address)).to
            .emit(genesisLock, "RightsChanging")
            .withArgs(addr0.address, addr4.address)

        await expect(genesisLock.connect(addr3).acceptAllRights(addr0.address)).to
            .revertedWith("no changing record")

        await expect(genesisLock.connect(addr4).acceptAllRights(addr0.address)).to
            .emit(genesisLock, "RightsAccepted")
            .withArgs(addr0.address, addr4.address)

        await expect(genesisLock.connect(addr4).acceptAllRights(addr1.address)).to
            .revertedWith("no changing record")
    });

    it('should not be changed after all lock-ups are claimed', async function () {
        genesisLock = genesisLock.connect(addr0);
        let addr = addr0.address;
        await hre.network.provider.send('evm_increaseTime', [4 * periodTime])
        await utils.mineEmptyBlock();
        await expect(genesisLock.claim()).to
            .emit(genesisLock, "ReleaseClaimed")
            .withArgs(addr, BigNumber.from(3), singleLockAmount);

        await expect(genesisLock.connect(addr0).changeAllRights(addr3.address)).to
            .revertedWith("all claimed, no need to do anything")
    });

    async function rightsCleared(addr) {
        const userinfo = await genesisLock.getUserInfo(addr);
        for (let i = 0; i < 6; i++) {
            expect(userinfo[i]).to.be.eq(0)
        }
    }
})
