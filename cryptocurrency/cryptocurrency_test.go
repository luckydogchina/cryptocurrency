package main

import (
	pb "github.com/hyperledger/fabric/protos/peer"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	"encoding/json"
	"utxo"
	"testing"
	"fmt"
)

func checkInit(t *testing.T, stub *shim.MockStub, args[][]byte)  {
	res := stub.MockInit("1", args);
	if res.Status != shim.OK {
		fmt.Println("Init failed", string(res.Message))
		t.FailNow()
	}
}

func checkInvoke(t *testing.T, stub *shim.MockStub, args[][]byte) pb.Response {
	res := stub.MockInvoke("1", args);

	if res.Status != shim.OK {
		fmt.Println("Invoke", args, "failed", string(res.Message))
		t.FailNow()
	}

	fmt.Printf("respond payload: %s\n", res.Payload);

	return res;
}


/*测试初始化*/
func TestCryptoCurrencyCode_Init(t *testing.T) {
	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);
	output,_ := utxo.GetCreatorId(stub);
	Tx, _ := utxo.MakeGenesisTx(output, 1000000000000.0);
	genesis,_:= json.Marshal(&Tx);
	checkInit(t, stub, [][]byte{genesis});


}

/*测试提交交易*/
func TestCryptoCurrency_Invoke(t *testing.T) {
	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);

	output,_ := utxo.GetCreatorId(stub);
	genesisTx, _ := utxo.MakeGenesisTx(output, 100000.0);
	genesis,_:= json.Marshal(&genesisTx);
	checkInit(t, stub, [][]byte{genesis});

	UnspentTx, balance,_ := utxo.MakeUtxo(output, 9999.99, []utxo.Tx{genesisTx});
	fmt.Printf("the expect balance is %f\n", balance);
	Unspent, _ := json.Marshal(&UnspentTx);
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});

}

/*测试查询交易*/
func TestCryptoCurrency_Query(t *testing.T) {

	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);

	output,_ := utxo.GetCreatorId(stub);
	genesisTx, _ := utxo.MakeGenesisTx(output, 100000.0);
	genesis,_:= json.Marshal(&genesisTx);
	checkInit(t, stub, [][]byte{genesis});

	UnspentTx, balance,_ := utxo.MakeUtxo(output, 9999.99, []utxo.Tx{genesisTx});
	fmt.Printf("the expect balance is %f\n", balance);
	Unspent, _ := json.Marshal(&UnspentTx);
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});

	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QueryAll)})
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QueryUnspent)})
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QuerySpent)})

	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery),[]byte(utxo.QueryById), utxo.GetTxId(&genesisTx)})

}



/*测试连续花费*/
func TestCryptoCurrency_Spend1(t *testing.T) {
	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);

	output,_ := utxo.GetCreatorId(stub);
	genesisTx, _ := utxo.MakeGenesisTx(output, 100000.0);
	genesis,_:= json.Marshal(&genesisTx);
	checkInit(t, stub, [][]byte{genesis});

	UnspentTx, balance,_ := utxo.MakeUtxo(output, 9999.99, []utxo.Tx{genesisTx});
	fmt.Printf("the expect balance is %f\n", balance);
	Unspent, _ := json.Marshal(&UnspentTx);
	res := checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QueryUnspent)})

	balanceTx := utxo.Tx{};
	json.Unmarshal(res.Payload, &balanceTx);

	UnspentTx2, balance, _ := utxo.MakeUtxo(output, 1000.0, []utxo.Tx{UnspentTx, balanceTx});
	fmt.Printf("the expect balance is %f\n", balance);
	Unspent2, _ := json.Marshal(&UnspentTx2);
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent2});
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QueryUnspent)})

}
/*测试空花*/
func TestCryptoCurrency_Spend3(t *testing.T) {
	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);

	UnspentTx := utxo.Tx{};
	UnspentTx.Output,_ = utxo.GetCreatorId(stub);
	UnspentTx.Inputs = [][]byte{[]byte(utxo.CoinBase)};
	UnspentTx.Fee = 10000.000
	Unspent, _ := json.Marshal(&UnspentTx);

	//执行本条语句会显示错误: xxx is not utxo, 因为输入地址无效
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});
}

/*测试重复花费*/
func TestCryptoCurrency_Spend2(t *testing.T) {
	scc := new(CryptoCurrency)
	stub := shim.NewMockStub("crypto", scc);

	output,_ := utxo.GetCreatorId(stub);
	genesisTx, _ := utxo.MakeGenesisTx(output, 100000.0);
	genesis,_:= json.Marshal(&genesisTx);
	checkInit(t, stub, [][]byte{genesis});

	UnspentTx, balance,_ := utxo.MakeUtxo(output, 9999.99, []utxo.Tx{genesisTx});
	fmt.Printf("the expect balance is %f\n", balance);
	Unspent, _ := json.Marshal(&UnspentTx);
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionQuery), []byte(utxo.QueryUnspent)});

	//执行本条语句会显示错误: xxx is not utxo, 因为上面已经花费过了
	checkInvoke(t, stub, [][]byte{[]byte(utxo.FunctionSpend), Unspent});
}



