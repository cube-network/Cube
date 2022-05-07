const hre = require("hardhat");
const {BigNumber} = require("ethers");


async function getLatestTimestamp() {
    let block = await hre.ethers.provider.send("eth_getBlockByNumber", ['latest', false])
    return block.timestamp;
}

async function getLatestCoinbase() {
    return await hre.ethers.provider.send("eth_coinbase",[])
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

function gweiToWei(value) {
    // Just because ether to Gwei is the same multiple as Gwei to Wei, this is lazy
    return hre.ethers.utils.parseUnits(BigNumber.from(value).toString(), "gwei");
}
module.exports = {getLatestTimestamp, getLatestCoinbase, ethToGwei, ethToWei, weiToGWei, weiToEth, gweiToWei, mineEmptyBlock}