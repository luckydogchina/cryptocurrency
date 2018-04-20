package main

import (
	"fmt"
	"utxo"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"encoding/json"
	"encoding/base64"
)


// SimpleChaincode example simple Chaincode implementation
type CryptoCurrency struct {
}

//按照不同的查询类型，查询反馈交易Id
/*
@param:
	stub： 略
	ouput: 交易输出地址，在本合约中强制为creator对应的地址
	code: 指定查询类型 0x01 未花费 0x10 已花费 0x11 所有交易
@return
	错误信息 or 交易id的队列
*/
func (c *CryptoCurrency) splitCompositeKeyQuery(stub shim.ChaincodeStubInterface, output []byte, code []byte) pb.Response {
	var txResult = [][]byte{};
	outputBase64 := base64.StdEncoding.EncodeToString(output);

	if txResultsIterator, err := stub.GetStateByPartialCompositeKey(utxo.IndexName, []string{outputBase64}); err != nil{
		return shim.Error(err.Error());
	}else {

		for txResultsIterator.HasNext(){
			if response , err := txResultsIterator.Next(); err != nil{
				break;
			}else {
				if response.Value == nil{
					return shim.Error(fmt.Sprintf("the value of key %s is not exist.", response.Key));
				}
				if (response.Value[0]&code[0]) != 0x00{
					indexType, compositeKeyParts, err := stub.SplitCompositeKey(response.Key);
					if err != nil{
						break;
					}
					fmt.Printf("- found a tx from index:%s output:%s id:%s\n", indexType, compositeKeyParts[0], compositeKeyParts[1]);

					lastIndex := len(txResult);
					txTemp:= make([][]byte,  lastIndex + 1);
					copy(txTemp, txResult);
					txTemp[lastIndex], _ = base64.StdEncoding.DecodeString(compositeKeyParts[1]);
					txResult = txTemp;
				}
			}
		}

		txResultsIterator.Close();
		if err != nil{
			return shim.Error(err.Error());
		}
	}

	result,_ := json.Marshal(&utxo.TxIdList{txResult});
	return shim.Success(result);
}

//查询交易;
/*
@param:
	stub: 略
	args ： args[1]:查询的类型， 目前支持根据交易id查询、查询creator所有未花费交易、查询creator所有花费交易、查询creator所有交易

@return:
	错误信息 or 查询结果
*/
func (c *CryptoCurrency) query(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response {
	argsLength := len(args);
	if argsLength < 2{
		return shim.Error("the paraments are not enough.");
	}

	queryType := string(args[1]);
	switch queryType {
	case utxo.QueryById:{
			if argsLength != 3{
				return shim.Error("the paraments are not matched");
			}

			txKey := string(args[2]);
			if txValue, err := stub.GetState(txKey); err != nil{
				return shim.Error(err.Error());
			}else {
				if txValue == nil{
					return shim.Error(fmt.Sprintf("the value of key %s is not exist: %s", txKey));
				}
				return shim.Success(txValue);
			}
		}
	case utxo.QueryUnspent:{
			output, err := utxo.GetCreatorId(stub);
			if err != nil{
				return shim.Error(err.Error());
			}
			return c.splitCompositeKeyQuery(stub, output[:], []byte{0x01});
		}

	case utxo.QuerySpent:{
			output, err := utxo.GetCreatorId(stub);
			if err != nil{
				return shim.Error(err.Error());
			}
			return c.splitCompositeKeyQuery(stub, output[:], []byte{0x10});
		}
	case utxo.QueryAll:{
			output, err := utxo.GetCreatorId(stub);
			if err != nil{
				return shim.Error(err.Error());
			}
			return c.splitCompositeKeyQuery(stub, output[:], []byte{0x11});

		}
	default:
		break;
	}

	return shim.Error(fmt.Sprintf("sorry, %s function is not supported.", queryType));
}

//花费交易
/*
@param:
	stub: 略
	args：arg[1] 客户端构造的utxo
@return：
	错误信息 or 找零 utxo
*/
func (c *CryptoCurrency) spend(stub shim.ChaincodeStubInterface, args [][]byte) pb.Response{
	var respondPayload []byte = nil;
	if len(args) < 2{
		return shim.Error("none tx is spent");
	}

	tx := utxo.Tx{};
	if err := json.Unmarshal(args[1], &tx); err != nil{
		return shim.Error(err.Error());
	}

	//检查输入的Tx
	if balanceTx, err := utxo.CheckInputTxs(stub, &tx); err != nil{
		return shim.Error(err.Error());
	}else {
		output,_ := utxo.GetCreatorId(stub);
		//修改InputTxs的状态为已经花费;
		//此处用来抗双花;
		for _, inputTxId := range tx.Inputs{
			//修改composite key的state为spent;
			if err = utxo.SpentCompositeKey(stub, output, inputTxId); err != nil{
				return shim.Error(err.Error());
			}
		}

		//写入新utxo
		utxoKey := utxo.GetTxId(&tx);
		utxoValue,_ := json.Marshal(&tx);
		if err = stub.PutState(string(utxoKey), utxoValue); err != nil{
			return shim.Error(err.Error());
		}

		//初始化utxo composite key
		if err = utxo.InitTxCompositeKey(stub, tx.Output, utxoKey); err != nil{
			return shim.Error(err.Error());
		}

		//写入找零交易
		if balanceTx == nil{
			return shim.Success(nil);
		}

		balanceTxKey := utxo.GetTxId(balanceTx);
		balanceTxValue,_ := json.Marshal(balanceTx);

		if err = stub.PutState(string(balanceTxKey), balanceTxValue); err != nil{
			shim.Error(err.Error());
		}

		//初始化balance composite key
		if err = utxo.InitTxCompositeKey(stub,  balanceTx.Output, balanceTxKey); err != nil{
			return shim.Error(err.Error());
		}

		respondPayload = balanceTxValue;
	}

	return shim.Success(respondPayload);
}

//初始化发行代币的总量
func (c *CryptoCurrency) Init(stub shim.ChaincodeStubInterface) pb.Response {
	var err error

	args := stub.GetArgs();

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1");
	}

	genesisTx := utxo.Tx{};
	if err = json.Unmarshal(args[0],&genesisTx); err != nil {
		return shim.Error(err.Error());
	}

	//校验creator与Tx.Output是否一致;
	if err = utxo.CheckOwner(stub, &genesisTx); err != nil{
		return shim.Error(err.Error());
	}

	if genesisTx.Fee <= 0 {
		return shim.Error("genesis tx fee is not valide");
	}

	//写入创世交易;
	genesisTxId := utxo.GetTxId(&genesisTx);

	if err = stub.PutState(string(genesisTxId), args[0]); err != nil {
		return shim.Error(err.Error())
	}
	//初始化composite key，用于复杂查询;
	if err = utxo.InitTxCompositeKey(stub, genesisTx.Output , genesisTxId); err != nil{
		return shim.Error(err.Error());
	}

	fmt.Printf("the genesis coin number: %f and Tx id : %s", genesisTx.Fee, string(genesisTxId));
	return shim.Success(nil)
}

//提交执行花费或查询交易的操作
func (c *CryptoCurrency) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	args := stub.GetArgs();
	if (args == nil){
		return shim.Error("the invoke paraments cannot be empty!!!");
	}

	function := string(args[0]);
	switch function {
	case utxo.FunctionSpend:
		return c.spend(stub, args);
	case utxo.FunctionQuery:
		return c.query(stub, args);
	default:
		break;
	}

	return shim.Error(function + " fuction is not support, sorry.")
}

func main() {
	err := shim.Start(new(CryptoCurrency))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}