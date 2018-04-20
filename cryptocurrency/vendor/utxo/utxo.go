package utxo

import (
	"github.com/hyperledger/fabric/core/chaincode/shim"
	"crypto/sha256"
	"fmt"
	"encoding/json"
	"encoding/base64"
)


const (
	CoinBase = "CoinBase"	//coinbase地址

	IndexName  = "output~txid"	//composite key

	//定义查询交易的类型
	QueryById  = "Identify"  //按照交易Id查询
	QuerySpent = "Spent"	//查询creator所有已经花费的交易
	QueryUnspent = "Unspent"	//查询creator所有未花费的交易
	QueryAll	= "All"	//查询creator 所有交易

	//定义invoke 函数
	FunctionSpend = "sepnd"	//花费
	FunctionQuery = "query"	//查询
)

//交易结构
type Tx struct {
	//包含输入的未花费交易Id;
	Inputs 	[][]byte `json:"input"`;
	//输出的地址,与bitcoin不同,一个Tx中只能指定一个输出地址;
	Output []byte	`json:"output"`;
	//包含的资金
	Fee 	float64	`json:"fee"`;
}

//交易Id列表
type TxIdList struct {
	TxID [][]byte `json:"tx_id"`;
}

//校验这笔交易是否为当前的creator所拥有
/*
	@param:
		stub：可以通过接口来读取creator信息
		tx:待检查的交易
	@return:
		错误信息
*/
func CheckOwner(stub shim.ChaincodeStubInterface, tx *Tx) error {

	ownerId, err := GetCreatorId(stub);
	if err != nil{
		return err;
	}
	if string(ownerId[:]) != string(tx.Output[:]){
		return fmt.Errorf("sorry, the tx does not belong to you!!!");
	}

	return nil
}

//检查输入交易是否合法;
/*
	@param：
		stub：可以获取交易发起人的信息
		tx： 待检查的交易
	@return：
		找零交易 | 错误信息
*/
func CheckInputTxs(stub shim.ChaincodeStubInterface, tx *Tx) (*Tx, error){
	var sum float64 = 0;

	for _, inputTxId := range tx.Inputs{
		inputTxKey := string(inputTxId[:]);
		inputTxValue, err := stub.GetState(inputTxKey);

		if err != nil{
			return nil, fmt.Errorf("the input tx is not exist: %s ", inputTxKey);
		}

		inputTx := Tx{};
		json.Unmarshal(inputTxValue, &inputTx);

		if err = CheckOwner(stub, &inputTx); err != nil{
			return nil, err;
		}


		if  Unspent, err := CheckTxUnSpentState(stub, inputTx.Output, GetTxId(&inputTx)); err == nil && Unspent{
			//累积输入的金额;
			sum+= inputTx.Fee;
		}else {
			if err != nil{
				return nil, err;
			}else {
				return nil, fmt.Errorf("sorry, this input %s is not utxo", inputTxKey);
			}

		}

	}

	if sum < tx.Fee{
		return nil,fmt.Errorf("sorry , your input is not enough, input: %f and output %f ", sum, tx.Fee);
	}

	//计算余额
	balance := sum -tx.Fee;
	if balance == 0{
		return nil, nil;
	}

	balanceOutput, err :=  GetCreatorId(stub);
	if err != nil{
		return nil, err;
	}

	//一笔找零交易不需要输入;
	balanceTx := Tx{Inputs: [][]byte{[]byte(CoinBase)}, Output: balanceOutput, Fee:balance};
	return &balanceTx, nil;
}


//获取当前用户对应的output地址
/*
	@param:
		stub: 略
	@return:
		creator对应的output地址 | 错误信息
*/
func GetCreatorId(stub shim.ChaincodeStubInterface )([]byte, error){

	if owner,err := stub.GetCreator(); err != nil{
		return nil,err;
	}else {
		sh256 := sha256.New();
		sh256.Write(owner);
		creatorId := sh256.Sum(nil);
		return creatorId, nil;
	}

	return nil, nil;
}

func GetTxId(tx *Tx) []byte {
	sh256 := sha256.New()
	txData,_ := json.Marshal(tx);
	sh256.Write(txData);
	return sh256.Sum(nil);
}

func MakeGenesisTx(output []byte, Fee float64) (Tx, error ){
	if Fee < 0 || output == nil{
		return Tx{}, fmt.Errorf("the paraments are not vaild");
	}

	return Tx{Inputs:[][]byte{[]byte(CoinBase)}, Output:output,  Fee:Fee}, nil;
}

func MakeUtxo(output []byte, outputFee float64, inputTxs []Tx) (Tx, float64, error) {
	var sum float64 = 0;

	if output == nil || outputFee <0 || len(inputTxs) == 0{
		return Tx{}, 0, fmt.Errorf("the paraments are not vaild");
	}

	for i, inputTx := range inputTxs{
		if inputTx.Fee < 0{
			return Tx{}, 0, fmt.Errorf("sorry, number %d tx is invaild", i);
		}

		sum += inputTx.Fee;
	}

	if sum < outputFee{
		return Tx{},0, fmt.Errorf("sorry, input %f is less than ouput %f.", sum, outputFee);
	}

	utxo := Tx{Output:output,Fee:outputFee};
	utxo.Inputs = make([][]byte, len(inputTxs));
	for i, inputTx := range inputTxs{
		 utxo.Inputs[i] = GetTxId(&inputTx);
	}

	return utxo, sum - outputFee, nil;
}

func compositeKey(stub shim.ChaincodeStubInterface, output []byte, txId []byte) (string, error) {
	outputBase64 := base64.StdEncoding.EncodeToString(output);
	txIdBase64 := base64.StdEncoding.EncodeToString(txId);
	return  stub.CreateCompositeKey(IndexName, []string{outputBase64, txIdBase64});
}

/*
	0x01 表示没有花费
	0x10 表示已经花费
*/

//创建一个compositeKey，并把value置为0x01，用来表示没有花费
/*
@param:
	stub: 略
	ouput： txId对应的输出地址
	txId: 交易id， 使用此函数时应传入utxoId
@return：
	错误信息
*/
func InitTxCompositeKey(stub shim.ChaincodeStubInterface, output []byte, txId []byte) error  {

	if outputTxidIndexKey, err := compositeKey(stub, output, txId); err != nil{
		return err;
	}else {
		return stub.PutState(outputTxidIndexKey, []byte{0x01});
	}

	return nil;
}

//把花费状态置为0x00，用来表示已经花费了的状态
/*
@param:
	stub: 略
	output： txId对应的交易输出地址
	txId: 交易Id， 使用此函数时应传入 被花费的Tx id
*/
func SpentCompositeKey(stub shim.ChaincodeStubInterface, output []byte, txId []byte) error  {
	if	outputTxidIndexKey, err := compositeKey(stub, output, txId);err != nil{
		return err;
	}else {
		if value, err := stub.GetState(outputTxidIndexKey); err != nil{
			return err;
		}else {
			if value == nil{
				return fmt.Errorf("the value of the composite key is not exist: %s", outputTxidIndexKey);
			}

			switch value[0] {
			case 0x01:
				err = stub.PutState(outputTxidIndexKey, []byte{0x10});
			case 0x10:
				err = nil;
			default:
				err = fmt.Errorf("the compostie key %s of tx %s is wrong %x", outputTxidIndexKey, txId, value[0]);
			}

			return err;
		}
	}

	return nil;
}

//检查交易是否未花费
/*
	@param:
		stub: 略
		output: 交易输出地址
		txId： 待检测交易的Id
	@return
		true：未花费 or false 已经花费 | 错误信息
*/
func CheckTxUnSpentState(stub shim.ChaincodeStubInterface, output []byte, txId []byte) (bool ,error)  {

	if outputTxidIndexKey, err := compositeKey(stub, output, txId); err != nil{
		return false, err;
	}else {
		if value, err := stub.GetState(outputTxidIndexKey);err != nil{
			return false, err;
		} else {
			if value == nil{
				return false, fmt.Errorf("the value of the composite key is not exist: %s", outputTxidIndexKey);
			}

			switch value[0] {
			case 0x01:
				return true, nil;
			case 0x10:
				return false, nil;
			default:
				break;
			}

			return false, fmt.Errorf("the compostie key %s of tx %s is wrong %x", outputTxidIndexKey, txId, value[0])
		}
	}

	return false, nil;
}