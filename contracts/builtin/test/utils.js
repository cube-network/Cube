const hre = require("hardhat");
const {BigNumber} = require("ethers");


async function getLatestTimestamp() {
    let block = await hre.ethers.provider.send("eth_getBlockByNumber", ['latest', false])
    return block.timestamp;
}

async function mineEmptyBlock() {
    await hre.ethers.provider.send("evm_mine");
}

function ethToGwei(value) {
    let gwei = hre.ethers.utils.parseUnits(BigNumber.from(value).toString(), "gwei");
    return gwei;
}

function ethToWei(value) {
    let wei = hre.ethers.utils.parseUnits(BigNumber.from(value).toString(), "ether");
    return wei;
}

function weiToEth(value) {
    let eth = hre.ethers.utils.formatUnits(BigNumber.from(value), "ether");
    return eth;
}
function weiToGWei(value) {
    let eth = hre.ethers.utils.formatUnits(BigNumber.from(value), "gwei");
    return eth;
}

module.exports = {getLatestTimestamp, ethToGwei, ethToWei, weiToGWei, weiToEth, mineEmptyBlock}