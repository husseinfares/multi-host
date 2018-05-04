package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
)

type SimpleChaincode struct {
}

type student struct {
	ObjectType string `json:"docType"` 
	Cert       string `json:"cert"`    
	Degree      string `json:"degree"`
	ID       int    `json:"iD"`
	Owner      string `json:"owner"`
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}


func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	fmt.Println("invoke is running " + function)

	
	if function == "initCert" { 
		return t.initCert(stub, args)
	}else if function == "readCert" { 
		return t.readCert(stub, args)
	} else if function == "queryCertByOwner" {
		return t.queryCertByOwner(stub, args)
	} 

	fmt.Println("invoke did not find func: " + function)
	return shim.Error("Received unknown function invocation")
}


func (t *SimpleChaincode) initCert(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error

	//   0       1       2     3
	// "as23df", "ME", "4674", "hussein"
	if len(args) != 4 {
		return shim.Error("Incorrect number of arguments. Expecting 4")
	}

	// ==== Input sanitation ====
	fmt.Println("- start init student")
	if len(args[0]) <= 0 {
		return shim.Error("1st argument must be a non-empty string")
	}
	if len(args[1]) <= 0 {
		return shim.Error("2nd argument must be a non-empty string")
	}
	if len(args[2]) <= 0 {
		return shim.Error("3rd argument must be a non-empty string")
	}
	if len(args[3]) <= 0 {
		return shim.Error("4th argument must be a non-empty string")
	}
	studentcert := args[0]
	degree := strings.ToLower(args[1])
	owner := strings.ToLower(args[3])
	iD, err := strconv.Atoi(args[2])
	if err != nil {
		return shim.Error("3rd argument must be a numeric string")
	}

	// ==== Check if student already exists ====
	studentAsBytes, err := stub.GetState(studentcert)
	if err != nil {
		return shim.Error("Failed to get student: " + err.Error())
	} else if studentAsBytes != nil {
		fmt.Println("This student already exists: " + studentcert)
		return shim.Error("This student already exists: " + studentcert)
	}

	// ==== Create student object and marshal to JSON ====
	objectType := "student"
	student := &student{objectType, studentcert, degree, iD, owner}
	studentJSONasBytes, err := json.Marshal(student)
	if err != nil {
		return shim.Error(err.Error())
	}
	//Alternatively, build the student json string manually if you don't want to use struct marshalling
	//studentJSONasString := `{"docType":"Student",  "name": "` + studentcert + `", "degree": "` + degree + `", "iD": ` + strconv.Itoa(iD) + `, "owner": "` + owner + `"}`
	//studentJSONasBytes := []byte(str)

	// === Save student to state ===
	err = stub.PutState(studentcert, studentJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	//  ==== Index the student to enable degree-based range queries, e.g. return all blue students ====
	//  An 'index' is a normal key/value entry in state.
	//  The key is a composite key, with the elements that you want to range query on listed first.
	//  In our case, the composite key is based on indexName~degree~name.
	//  This will enable very efficient state range queries based on composite keys matching indexName~degree~*
	indexName := "degree~name"
	colorNameIndexKey, err := stub.CreateCompositeKey(indexName, []string{student.Degree, student.Cert})
	if err != nil {
		return shim.Error(err.Error())
	}
	//  Save index entry to state. Only the key name is needed, no need to store a duplicate copy of the student.
	//  Note - passing a 'nil' value will effectively delete the key from state, therefore we pass null character as value
	value := []byte{0x00}
	stub.PutState(colorNameIndexKey, value)

	// ==== Student saved and indexed. Return success ====
	fmt.Println("- end init student")
	return shim.Success(nil)
}

func (t *SimpleChaincode) readCert(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var cert, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting cert of the student to query")
	}

	cert = args[0]
	valAsbytes, err := stub.GetState(cert)
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + cert + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"student does not exist: " + cert + "\"}"
		return shim.Error(jsonResp)
	}

	return shim.Success(valAsbytes)
}

func (t *SimpleChaincode) queryCertByOwner(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	//   0
	// "bob"
	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	owner := strings.ToLower(args[0])

	queryString := fmt.Sprintf("{\"selector\":{\"docType\":\"student\",\"owner\":\"%s\"}}", owner)

	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	fmt.Printf("- getQueryResultForQueryString queryString:\n%s\n", queryString)

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	// buffer is a JSON array containing QueryRecords
	var buffer bytes.Buffer
	buffer.WriteString("[")

	bArrayMemberAlreadyWritten := false
	for resultsIterator.HasNext() {
		queryResponse, err := resultsIterator.Next()
		if err != nil {
			return nil, err
		}
		// Add a comma before array members, suppress it for the first array member
		if bArrayMemberAlreadyWritten == true {
			buffer.WriteString(",")
		}
		buffer.WriteString("{\"Key\":")
		buffer.WriteString("\"")
		buffer.WriteString(queryResponse.Key)
		buffer.WriteString("\"")

		buffer.WriteString(", \"Record\":")
		// Record is a JSON object, so we write as-is
		buffer.WriteString(string(queryResponse.Value))
		buffer.WriteString("}")
		bArrayMemberAlreadyWritten = true
	}
	buffer.WriteString("]")

	fmt.Printf("- getQueryResultForQueryString queryResult:\n%s\n", buffer.String())

	return buffer.Bytes(), nil
}
