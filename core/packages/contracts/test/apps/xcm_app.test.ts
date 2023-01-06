import { defaultAbiCoder } from "@ethersproject/abi"
import { expect, loadFixture } from "../setup"
import { xcmAppFixture } from "./fixtures"

let POLKADOT_ORIGIN = "0xd43593c715fdd31c61141abd04a99fd6822c8558854ccde39a5684e7a56da27d"

describe("XCMApp", function () {
    describe("Proxies", function () {
        it("downstream sees proxy as msg.sender", async function () {
            let { app, executor, assetManager, downstream, user } = await loadFixture(xcmAppFixture)
            let proxy = "0xe1d2a389cd3e9694D374507E00C49d643605a2fb"
            let abi = defaultAbiCoder

            let encodedFunc = downstream.interface.encodeFunctionData("recordMsgSender")

            // Xcm Transact
            let transact = abi.encode(
                ["tuple(address, bytes)"],
                [[downstream.address, encodedFunc]]
            )

            let expectedEncodedCall = executor.interface.encodeFunctionData("execute", [
                assetManager.address,
                [{ kind: 0, arguments: transact }],
            ])

            let hardcoded_payload =
                "0x0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000774667629726ec1fabebcec0d9139bd1c8f72a230000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000458d524e100000000000000000000000000000000000000000000000000000000"

            //let payload = abi.encode(["tuple(uint8 kind,bytes arguments)[]"],[
            //    [ { kind: 0, arguments: transact} ]
            //]);
            //await expect(payload, "payload is created successfully").to.eq(hardcoded_payload);

            await expect(
                app.dispatchToProxy(POLKADOT_ORIGIN, executor.address, hardcoded_payload, {
                    gasLimit: 1_000_000,
                })
            )
                .to.emit(app, "XcmExecuted")
                .withArgs(
                    POLKADOT_ORIGIN,
                    proxy,
                    executor.address,
                    true,
                    "0x8e1b5c0e",
                    "0x000000000000000000000000eda338e4dc46038493b885327842fd3e301cab39",
                    "0x0000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000a00000000000000000000000000000000000000000000000000000000000000020000000000000000000000000774667629726ec1fabebcec0d9139bd1c8f72a230000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000000458d524e100000000000000000000000000000000000000000000000000000000",
                    expectedEncodedCall
                )
                .to.emit(downstream, "RecordSender")
                .withArgs(proxy)
        })
    })
})
