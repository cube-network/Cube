require("@nomiclabs/hardhat-solpp");
require("hardhat-contract-sizer");
require("@nomiclabs/hardhat-waffle");
require("@nomiclabs/hardhat-truffle5");
// This is a sample Hardhat task. To learn how to create your own go to
// https://hardhat.org/guides/create-task.html
task("accounts", "Prints the list of accounts", async (taskArgs, hre) => {
  const accounts = await hre.ethers.getSigners();

  for (const account of accounts) {
    console.log(account.address);
  }
});

// You need to export an object to set up your config
// Go to https://hardhat.org/config/ to learn more

const prodConfig = {
  Mainnet: true,
}

const devConfig = {
  Mainnet: false,
}

const contractDefs = {
  mainnet: prodConfig,
  devnet: devConfig
}

module.exports = {
  solidity: {
    version: "0.8.4",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200
      }
    }
  },
  solpp: {
    defs: contractDefs[process.env.NET]
  },
  networks: {
    hardhat: {
      allowUnlimitedContractSize: true,
      accounts: {
        mnemonic: "test test test test test test test test test test test junk",
        count: 100,
        accountsBalance: "1000000000000000000000000000"
      },
      hardfork: "berlin"
    }
  }
};
