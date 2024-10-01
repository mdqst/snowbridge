module.exports = {
  apps: [
    {
      name: "monitor",
      node_args: "--require=dotenv/config",
      script: "./dist/src/main.js",
      args: "cron"
    },
    {
      name: "transferToPolkadot",
      node_args: "--require=dotenv/config",
      script: "./dist/src/transfer_to_polkadot.js",
      args: "cron"
    },
    {
      name: "transferToEthereum",
      node_args: "--require=dotenv/config",
      script: "./dist/src/transfer_to_ethereum.js",
      args: "cron"
    },
  ],
};
