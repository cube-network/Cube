const {ether, expectEvent, expectRevert, BN} = require("@openzeppelin/test-helpers");

const AddressList = artifacts.require("AddressList");
const Direction = {
    From: "0",
    To: "1",
    Both: "2"
}
describe("AddressList contract", function () {
    let accounts;
    let admin;
    let aList;
    let erc20transRule = {
        sig: "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
        idx: new BN(1),
        ct: '1'
    };
    // {0x06b541ddaa720db2b10a4d0cdac39b8d360425fc073085fac19bc82614677987,2,1}
    let erc777SentRule = {
        sig: "0x06b541ddaa720db2b10a4d0cdac39b8d360425fc073085fac19bc82614677987",
        idx: new BN(2),
        ct: '1'
    };
    // [{0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62,2,1},{0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb,2,1}]
    let erc1155transSingleRule = {
        sig: "0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62",
        idx: new BN(2),
        ct: '1'
    };
    let erc1155transBatchRule = {
        sig: "0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb",
        idx: new BN(2),
        ct: '1'
    };

    before(async function () {
        accounts = await web3.eth.getAccounts();
        admin = accounts[0];

        aList = await AddressList.new();
        await aList.initialize(admin);
    });

    it('should only init once', async function () {
        await expectRevert(aList.initialize(accounts[1]), "E41");
    });

    describe("manage black addresses", async function () {
        it('should can not add admin to blacklist', async function () {
            await expectRevert(aList.addBlacklist(admin, Direction.Both, {from: admin}), "cannot add admin to blacklist");
        });
        it('should only admin can add or remove', async function () {
            await expectRevert(aList.addBlacklist(accounts[1], Direction.From, {from: accounts[2]}), "Admin only")
            await expectRevert(aList.removeBlacklist(accounts[1], Direction.From, {from: accounts[2]}), "Admin only")
        });
        it('new add type from, query, and then remove', async function () {
            await addCheckAndRemove(accounts[1], Direction.From, "already in from list")
        });
        it('new add type to, query, and then remove', async function () {
            await addCheckAndRemove(accounts[1], Direction.To, "already in to list")
        });
        it('new add type both, query, and then remove', async function () {
            await addCheckAndRemove(accounts[1], Direction.Both, "already in both list")
        });

        async function addCheckAndRemove(acc, direction, alreadyMsg) {
            await addBlack(acc, direction);

            // can not add the same again
            await expectRevert(aList.addBlacklist(acc, direction, {from: admin}), alreadyMsg);

            let result = await aList.isBlackAddress(acc);
            assert.equal(result[0], true)
            assert.equal(result[1], direction)

            removeBlack(acc, direction, direction, direction === Direction.Both)
        }

        async function addBlack(acc, direction) {
            let receipt = await aList.addBlacklist(acc, direction, {from: admin})
            expectEvent(receipt, "BlackAddrAdded", {
                addr: acc,
                d: direction
            });
            await checkLastUpdate(receipt, false);
        }

        async function removeBlack(acc, opDire, evDire, isBothEvent) {
            let receipt = await aList.removeBlacklist(acc, opDire, {from: admin});
            if (isBothEvent) {
                expectEvent(receipt, "BlackAddrRemoved", {
                    addr: acc,
                    d: Direction.From
                });
                expectEvent(receipt, "BlackAddrRemoved", {
                    addr: acc,
                    d: Direction.To
                });
            } else {
                expectEvent(receipt, "BlackAddrRemoved", {
                    addr: acc,
                    d: evDire
                });
            }
            await checkLastUpdate(receipt, false);
        }

        it('should be ok: add from or add to, then add both', async function () {
            await addBlack(accounts[1], Direction.From);
            await addBlack(accounts[2], Direction.To);
            await addBlack(accounts[1], Direction.Both);
            await addBlack(accounts[2], Direction.Both);

            let result = await aList.isBlackAddress(accounts[1]);
            assert.equal(result[0], true)
            assert.equal(result[1], Direction.Both)
            result = await aList.isBlackAddress(accounts[2]);
            assert.equal(result[0], true)
            assert.equal(result[1], Direction.Both)

            removeBlack(accounts[1], Direction.From, Direction.From, false)
            removeBlack(accounts[1], Direction.To, Direction.To, false)
            removeBlack(accounts[2], Direction.Both, Direction.Both, true)
        });

        it('should be ok: add from or add to, then remove both', async function () {
            await addBlack(accounts[1], Direction.From);
            await addBlack(accounts[2], Direction.To);

            removeBlack(accounts[1], Direction.Both, Direction.From, false)
            removeBlack(accounts[2], Direction.Both, Direction.To, false)
        });

        it('should be ok: add both, then remove separately', async function () {
            await addBlack(accounts[1], Direction.Both);

            removeBlack(accounts[1], Direction.From, Direction.From, false)
            removeBlack(accounts[1], Direction.To, Direction.To, false)
        });
    })

    describe("manage rules", async function () {

        it('remove default rules for later test', async function () {
            //remove all rules for later test
            await aList.removeRule(erc1155transBatchRule.sig, erc1155transBatchRule.idx, {from: admin});
            await aList.removeRule(erc1155transSingleRule.sig, erc1155transSingleRule.idx, {from: admin});
            await aList.removeRule(erc777SentRule.sig, erc777SentRule.idx, {from: admin});
            await aList.removeRule(erc20transRule.sig, erc20transRule.idx, {from: admin});

            let len = await aList.rulesLen();
            assert.equal(0, len);
        });

        it('should only the admin can manage the rules', async function () {
            await expectRevert(aList.addOrUpdateRule(erc20transRule.sig, erc20transRule.idx, 2, {from: accounts[1]}), "Admin only");
            let receipt = await aList.addOrUpdateRule(erc20transRule.sig, erc20transRule.idx, 2, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc20transRule.sig,
                checkIdx: erc20transRule.idx,
                t: '2'
            });
            await expectRevert(aList.removeRule(erc20transRule.sig, erc20transRule.idx, {from: accounts[1]}), "Admin only");
            receipt = await aList.removeRule(erc20transRule.sig, erc20transRule.idx, {from: admin});
            expectEvent(receipt, "RuleRemoved", {
                eventSig: erc20transRule.sig,
                checkIdx: erc20transRule.idx,
                t: '2'
            });
        });

        it('should add rules correctly', async function () {

            await expectRevert(aList.addOrUpdateRule("0x0000000000000000000000000000000000000000000000000000000000000000", 0, 1), "eventSignature must not empty");
            await expectRevert(aList.addOrUpdateRule(erc20transRule.sig, 0, 1), "check index must greater than 0");
            await expectRevert(aList.addOrUpdateRule(erc20transRule.sig, 1, 0), "invalid check type");


            let receipt = await aList.addOrUpdateRule(erc20transRule.sig, erc20transRule.idx, 2, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc20transRule.sig,
                checkIdx: erc20transRule.idx,
                t: '2'
            });
            await checkLastUpdate(receipt, true)

            receipt = await aList.addOrUpdateRule(erc20transRule.sig, erc20transRule.idx, erc20transRule.ct, {from: admin});
            expectEvent(receipt, "RuleUpdated", {
                eventSig: erc20transRule.sig,
                checkIdx: erc20transRule.idx,
                t: erc20transRule.ct
            });
            await checkLastUpdate(receipt, true)

            receipt = await aList.addOrUpdateRule(erc777SentRule.sig, erc777SentRule.idx, erc777SentRule.ct, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc777SentRule.sig,
                checkIdx: erc777SentRule.idx,
                t: erc777SentRule.ct
            });
            await checkLastUpdate(receipt, true)

            receipt = await aList.addOrUpdateRule(erc1155transSingleRule.sig, erc1155transSingleRule.idx, erc1155transSingleRule.ct, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc1155transSingleRule.sig,
                checkIdx: erc1155transSingleRule.idx,
                t: erc1155transSingleRule.ct
            });
            await checkLastUpdate(receipt, true)

            receipt = await aList.addOrUpdateRule(erc1155transBatchRule.sig, erc1155transBatchRule.idx, erc1155transBatchRule.ct, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc1155transBatchRule.sig,
                checkIdx: erc1155transBatchRule.idx,
                t: erc1155transBatchRule.ct
            });
            await checkLastUpdate(receipt, true)

            let len = await aList.rulesLen();
            assert.equal(4, len);

        });

        it('should remove rules correctly', async function () {
            let receipt = await aList.removeRule(erc777SentRule.sig, erc777SentRule.idx, {from: admin});
            expectEvent(receipt, "RuleRemoved", {
                eventSig: erc777SentRule.sig,
                checkIdx: erc777SentRule.idx,
                t: erc777SentRule.ct
            });
            let lastUpdated = await aList.rulesLastUpdatedNumber();
            assert.equal(lastUpdated, receipt.receipt.blockNumber);

            receipt = await aList.addOrUpdateRule(erc777SentRule.sig, erc777SentRule.idx, erc777SentRule.ct, {from: admin});
            expectEvent(receipt, "RuleAdded", {
                eventSig: erc777SentRule.sig,
                checkIdx: erc777SentRule.idx,
                t: erc777SentRule.ct
            });
            lastUpdated = await aList.rulesLastUpdatedNumber();
            assert.equal(lastUpdated, receipt.receipt.blockNumber);

            receipt = await aList.removeRule(erc777SentRule.sig, erc777SentRule.idx, {from: admin});
            expectEvent(receipt, "RuleRemoved", {
                eventSig: erc777SentRule.sig,
                checkIdx: erc777SentRule.idx,
                t: erc777SentRule.ct
            });
            lastUpdated = await aList.rulesLastUpdatedNumber();
            assert.equal(lastUpdated, receipt.receipt.blockNumber);

            let len = await aList.rulesLen();
            assert.equal(3, len);

        });

        it('should be queryable', async function () {
            let len = await aList.rulesLen();
            for (let i = 0; i < len; i++) {
                let rule = await aList.getRuleByIndex(i);
                assert.notEqual(rule[0], '0x0000000000000000000000000000000000000000000000000000000000000000')
            }

            let rule = await aList.getRuleByKey(erc20transRule.sig, erc20transRule.idx);
            assert.equal(rule[0], erc20transRule.sig);
            assert.equal(rule[1].eq(erc20transRule.idx), true);
            assert.equal(rule[2], erc20transRule.ct);
        });
    })

    async function checkLastUpdate(receipt, isRules) {
        let lastUpdated = 0;
        if (isRules) {
            lastUpdated = await aList.rulesLastUpdatedNumber();
        } else {
            lastUpdated = await aList.blackLastUpdatedNumber();
        }
        assert.equal(lastUpdated, receipt.receipt.blockNumber);
    }
});