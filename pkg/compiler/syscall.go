package compiler

var syscalls = map[string]map[string]string{
	"account": {
		"GetBalance":    "Neo.Account.GetBalance",
		"GetScriptHash": "Neo.Account.GetScriptHash",
		"GetVotes":      "Neo.Account.GetVotes",
		"IsStandard":    "Neo.Account.IsStandard",
	},
	"storage": {
		"GetContext": "Neo.Storage.GetContext",
		"Put":        "Neo.Storage.Put",
		"Get":        "Neo.Storage.Get",
		"Delete":     "Neo.Storage.Delete",
		"Find":       "Neo.Storage.Find",
	},
	"runtime": {
		"GetTrigger":   "Neo.Runtime.GetTrigger",
		"CheckWitness": "Neo.Runtime.CheckWitness",
		"Notify":       "Neo.Runtime.Notify",
		"Log":          "Neo.Runtime.Log",
		"GetTime":      "Neo.Runtime.GetTime",
		"Serialize":    "Neo.Runtime.Serialize",
		"Deserialize":  "Neo.Runtime.Deserialize",
	},
	"blockchain": {
		"GetHeight":      "Neo.Blockchain.GetHeight",
		"GetHeader":      "Neo.Blockchain.GetHeader",
		"GetBlock":       "Neo.Blockchain.GetBlock",
		"GetTransaction": "Neo.Blockchain.GetTransaction",
		"GetContract":    "Neo.Blockchain.GetContract",
		"GetAccount":     "Neo.Blockchain.GetAccount",
		"GetValidators":  "Neo.Blockchain.GetValidators",
		"GetAsset":       "Neo.Blockchain.GetAsset",
	},
	"header": {
		"GetIndex":         "Neo.Header.GetIndex",
		"GetHash":          "Neo.Header.GetHash",
		"GetPrevHash":      "Neo.Header.GetPrevHash",
		"GetTimestamp":     "Neo.Header.GetTimestamp",
		"GetVersion":       "Neo.Header.GetVersion",
		"GetMerkleRoot":    "Neo.Header.GetMerkleRoot",
		"GetConsensusData": "Neo.Header.GetConsensusData",
		"GetNextConsensus": "Neo.Header.GetNextConsensus",
	},
	"block": {
		"GetTransactionCount": "Neo.Block.GetTransactionCount",
		"GetTransactions":     "Neo.Block.GetTransactions",
		"GetTransaction":      "Neo.Block.GetTransaction",
	},
	"transaction": {
		"GetHash":         "Neo.Transaction.GetHash",
		"GetType":         "Neo.Transaction.GetType",
		"GetAttributes":   "Neo.Transaction.GetAttributes",
		"GetInputs":       "Neo.Transaction.GetInputs",
		"GetOutputs":      "Neo.Transaction.GetOutputs",
		"GetReferences":   "Neo.Transaction.GetReferences",
		"GetUnspentCoins": "Neo.Transaction.GetUnspentCoins",
		"GetScript":       "Neo.Transaction.GetScript",
	},
	"asset": {
		"Create":       "Neo.Asset.Create",
		"GetAdmin":     "Neo.Asset.GetAdmin",
		"GetAmount":    "Neo.Asset.GetAmount",
		"GetAssetID":   "Neo.Asset.GetAssetID",
		"GetAssetType": "Neo.Asset.GetAssetType",
		"GetAvailable": "Neo.Asset.GetAvailable",
		"GetIssuer":    "Neo.Asset.GetIssuer",
		"GetOwner":     "Neo.Asset.GetOwner",
		"GetPrecision": "Neo.Asset.GetPrecision",
		"Renew":        "Neo.Asset.Renew",
	},
	"contract": {
		"GetScript":         "Neo.Contract.GetScript",
		"IsPayable":         "Neo.Contract.IsPayable",
		"Create":            "Neo.Contract.Create",
		"Destroy":           "Neo.Contract.Destroy",
		"Migrate":           "Neo.Contract.Migrate",
		"GetStorageContext": "Neo.Contract.GetStorageContext",
	},
	"input": {
		"GetHash":  "Neo.Input.GetHash",
		"GetIndex": "Neo.Input.GetIndex",
	},
	"output": {
		"GetAssetID":    "Neo.Output.GetAssetID",
		"GetValue":      "Neo.Output.GetValue",
		"GetScriptHash": "Neo.Output.GetScriptHash",
	},
	"engine": {
		"GetScriptContainer":     "System.ExecutionEngine.GetScriptContainer",
		"GetCallingScriptHash":   "System.ExecutionEngine.GetCallingScriptHash",
		"GetEntryScriptHash":     "System.ExecutionEngine.GetEntryScriptHash",
		"GetExecutingScriptHash": "System.ExecutionEngine.GetExecutingScriptHash",
	},
	"iterator": {
		"Create": "Neo.Iterator.Create",
		"Key":    "Neo.Iterator.Key",
		"Keys":   "Neo.Iterator.Keys",
		"Next":   "Neo.Iterator.Next",
		"Value":  "Neo.Iterator.Value",
		"Values": "Neo.Iterator.Values",
	},
}
