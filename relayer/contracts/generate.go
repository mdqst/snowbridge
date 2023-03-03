//go:generate bash -c "jq .abi ../../core/packages/contracts/out/OpaqueProof.sol/OpaqueProof.json | abigen --abi - --type OpaqueProof --pkg opaqueproof --out opaqueproof/contract.go"
//go:generate bash -c "jq .abi ../../core/packages/contracts/out/BeefyClient.sol/BeefyClient.json | abigen --abi - --type BeefyClient --pkg beefyclient --out beefyclient/contract.go"
//go:generate bash -c "jq .abi ../../core/packages/contracts/out/BasicInboundChannel.sol/BasicInboundChannel.json | abigen --abi - --type BasicInboundChannel --pkg basic --out basic/inbound.go"
//go:generate bash -c "jq .abi ../../core/packages/contracts/out/BasicOutboundChannel.sol/BasicOutboundChannel.json | abigen --abi - --type BasicOutboundChannel --pkg basic --out basic/outbound.go"

package contracts
