package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	. "fmt"
	"github.com/hyperledger/fabric/core/chaincode/shim"
	pb "github.com/hyperledger/fabric/protos/peer"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleModel struct {}

//storage for model
type Model struct {
	ObjectType 		string
	Name       		string
	Parameters 		[]float64 `json:"Parameters"`
	Owner      		string

}

type ModelFile struct{
	ObjectType 	string
	Name string
	File string
	Owner string
	ModelType		string `json:"ModelType"`
	LibraryType string `json:"LibraryType"`
	ID uint64
}

type DataFlex struct{
	ObjectType 	string `json:"ObjectType"`
	Data [][]string `json:"DataTable"`
	Class []string `json:"Class"`
	Owner string `json:"Owner"`
	DataName string  `json:"DataName"`
	ID uint64   `json:"Id"`
}

type DataCol struct{
	ObjectType 	string `json:"ObjectType"`
	XData []string `json:"xData"`
	YData []string `json:"yData"`
	Class []string `json:"Class"`
	Owner string `json:"Owner"`
	DataName string  `json:"DataName"`
	ID uint64   `json:"Id"`
}

type Payload struct {
	Data []byte     `json:"Data"`
	Model []byte    `json:"Model"`
}

type FilePayload struct{
	Data DataFlex `json:"Data"`
	Model ModelFile `json:"Model"`
}


type ModelWrapper struct{
	Key   string `json:"Key"`
	Record ModelFile `json:"Record"`
}


/*type DataWrapper struct{
	Key    string `json:"Key"`
	Record Data   `json:"Record"`
}*/

type DataColWrapper struct{
	Key    string 	`json:"Key"`
	Record DataCol 	`json:"Record"`
}

type DataFlexWrapper struct{
	Key    string 	`json:"Key"`
	Record DataFlex `json:"Record"`
}

type ResultsWrapper struct{
	Key    string 	`json:"Key"`
	Record ResultsArray 	`json:"Record"`
}

// storage for model results
type Results struct{
	ArrayOfResults     []float64 `json:"Results"`
}

type ResultsArray struct{
	ObjectType 	string `json:"ObjectType"`
	id int64
	Results []float64 `json:"Results"`
	ModelName string `json:"ModelName"`
	DataColName string `json:"DataColName"`
}

type ModelValidity struct{
	ModelValidity    int64 	`json:"modelValidity"`
}

var resIdCounter int64 = 0

// ===================================================================================
// Main chaincode
// ===================================================================================
func main() {
	if err := shim.Start(new(SimpleModel)); err != nil {
		Printf("Error starting SimpleAsset chaincode: %s", err)
	}
}

// Init initializes chaincode
// ===========================
func (t *SimpleModel) Init(stub shim.ChaincodeStubInterface) pb.Response {
	return shim.Success(nil)
}

// Lists functions available
func (t *SimpleModel) Invoke(stub shim.ChaincodeStubInterface) pb.Response {
	function, args := stub.GetFunctionAndParameters()
	Println("invoke is running " + function)

	// Handle different functions
	if function == "initModel" { //create a new model
		return t.initModel(stub, args)
	} else if function == "readModel" { //read one model from chaincode stateDB
		return t.readModel(stub, args)
	}else if function == "GetModelID" { //read one model from chaincode stateDB
		return t.GetModelID(stub, args)
	}else if function == "GetDataID" { //read one model from chaincode stateDB
		return t.GetDataID(stub, args)
	}else if function == "GetAllModels" { //read all models from chaincode couchDB
		return t.getAllModels(stub, args)
	}else if function == "GetAllData" { //read all models from chaincode couchDB
		return t.getAllData(stub, args)
	}else if function == "GetAllResults" { //read all models from chaincode couchDB
		return t.getAllResults(stub, args)
	}else if function == "initModelFile" { //read one model from chaincode stateDB
		return t.initModelFile(stub, args)
	}else if function == "testModelFile" { //read one model from chaincode stateDB
		return t.testModelFile(stub, args)
	}else if function == "initDataFile" { //read one model from chaincode stateDB
		return t.initDataFile(stub, args)
	}else if function == "initFlexData" { //init test data from cli console arguments
		return t.initFlexData(stub, args)
	}else if function == "insertedModelFile" { //read all data owned by same owner
		return t.insertedModelFile(stub, args)
	}else if function == "insertedDataFile" { //read all data owned by same owner
		return t.insertedDataFile(stub, args)
	}else if function == "queryDataByOwner" { //read all data owned by same owner
		return t.queryDataByOwner(stub, args)
	}else if function == "validateModel" { //validate single Model
		return t.validateModel(stub, args)
	}else if function == "validateModelFileAPI" { //validate single Model via Oracle
		return t.validateModelFileAPI(stub, args)
	}else if function == "updateAllModels" { //validate Many Models
		return t.updateAllModels(stub, args)
	}else if function == "updateAllModelsAPI" { //validate Many Models via Oracle
		return t.updateAllModelsAPI(stub, args)
	}else if function == "testConnection" { //validate Many Models via Oracle
		return t.testConnection(stub, args)
	}

	Println("invoke did not find func: " + function) //error
	return shim.Error("Received unknown function invocation")
}

// Methods for single Model validation -------------------------------------------------------------------------------
func (t *SimpleModel) validateModel(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var data []DataColWrapper
	var currentModel Model

	modelName:= args[0]
	dataOwner:= args[1]

	//getting data stored in couchDB
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"dataColumns\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	json.Unmarshal((queryResults), &data)
	//--------------------------------------------------
	//getting model stored in couchDB
	valAsbytes, err := stub.GetState(modelName)
	json.Unmarshal((valAsbytes), &currentModel)
	//--------------------------------------------------

	var arrayOfResults []float64
	for i := 0; i < len(data); i++ {
		for j:=0 ; j < len(data[i].Record.Class); j++ {
			result := calculateLogisticModelResults(data[i].Record.XData[j],data[i].Record.YData[j], currentModel)
			arrayOfResults = append(arrayOfResults, result)
		}
	}
	t.initResults(stub,args, modelName,data[0].Record.DataName, arrayOfResults)
	return shim.Success(nil)
}

/*func (t *SimpleModel) validateModelAPI(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	modelName:= args[0]
	dataOwner:= args[1]
	var results Results
	var payload Payload

	//getting model stored in couchDB-------------------
	modelJson, err := stub.GetState(modelName)
	//--------------------------------------------------
	//getting data stored in couchDB--------------------
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"data\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	dataJson, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	//--------------------------------------------------

	// setting the ip address of API
	ip :="http://192.168.144.0:8081/"

	//get validation results---------------
	url := ip + "apiValidate"

	payload.Model = modelJson
	payload.Data = dataJson

	payloadbytes, err := json.Marshal(payload)

	responseBytes := HttpPost(url,payloadbytes)

	json.Unmarshal(responseBytes, &results)

	t.initResults(stub,args, modelName, results.ArrayOfResults)
	return shim.Success(nil)
}*/

func (t *SimpleModel) validateModelFileAPI(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	modelName:= args[0]
	dataColId:= args[1]
	var results Results
	var payload FilePayload
	var modelJson ModelFile
	var data DataFlex

	//getting model stored in couchDB-------------------
	modelBytes, err := stub.GetState(modelName)
	if err != nil {
		return shim.Error(err.Error())
	}
	//--------------------------------------------------
	//getting data stored in couchDB--------------------
	dataBytes, err := stub.GetState(dataColId)
	if err != nil {
		return shim.Error(err.Error())
	}
	//--------------------------------------------------

	// setting the ip address of API
	ip :="http://192.168.144.2:8080/"

	json.Unmarshal((dataBytes), &data)
	if err != nil {
		return shim.Error(err.Error())
	}
	//--------------------------------------------------

	json.Unmarshal((modelBytes), &modelJson)
	if err != nil {
		return shim.Error(err.Error())
	}

	payload.Model = modelJson
	payload.Data = data

	//get validation results---------------
	url := ip + "apiValidate"+ modelJson.ModelType

	payloadJson, err := json.Marshal(payload)

	responseBytes := HttpPost(url,payloadJson)

	err = json.Unmarshal(responseBytes, &results)
	if err != nil {
		return shim.Error(err.Error())
	}
	t.initResults(stub,args, modelName, data.DataName ,results.ArrayOfResults )
	return shim.Success(nil)
}

func (t *SimpleModel) insertedModelFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	modelName:= args[0]
	var results Results
	var payload FilePayload
	var modelJson ModelFile

	//--------------------------------------------------

	// setting the ip address of API
	SparkIp :="http://192.168.144.2:8080/"
	MLR3Ip := "http://192.168.144.3:1030/"

	//--------------------------------------------------

	//getting model stored in couchDB-------------------
	modelBytes, err := stub.GetState(modelName)
	if err != nil {
		return shim.Error(err.Error())
	}

	err =json.Unmarshal((modelBytes), &modelJson)
	if err != nil {
		return shim.Error(err.Error())
	}
	payload.Model = modelJson

	//--------------------------------------------------
	//getting data stored in couchDB--------------------
	var wrappedData[]DataFlexWrapper
	queryString := "{\"selector\":{\"ObjectType\": \"dataColumns\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}"
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(queryResults, &wrappedData)
	if err != nil {
		return shim.Error(err.Error())
	}

	url := SparkIp+"apiValidate"+ modelJson.ModelType
	if modelJson.LibraryType == "MLR3"{
		url = MLR3Ip + "apiValidate"
	}
	//get validation results for each data---------------
	for i := 0; i < len(wrappedData); i++ {
		currentData := wrappedData[i].Record
		payload.Data = 	currentData

		payloadJson, err := json.Marshal(payload)
		responseBytes := HttpPost(url,payloadJson)
		if modelJson.LibraryType == "AS" {
			err = json.Unmarshal(responseBytes, &results)
			if err != nil {
				return shim.Error(err.Error())
			}
		}
		if modelJson.LibraryType == "MLR3" {
			stringFromBytes := string(responseBytes)
			cleanString := stringFromBytes
			cleanString = cleanString[1:len(cleanString)-1]
			s := strings.Split(cleanString, ",")
			floatResultsArray := []float64{}
				for i := 0; i < len(s); i++ {
					if i % 2 != 0{
						currentString := strings.Replace(s[i], "[", "", -1)
						currentString = strings.Replace(currentString, "]", "", -1)
						if n, err := strconv.ParseFloat(currentString, 64); err == nil {
							floatResultsArray = append(floatResultsArray, n)
						}
					}
				}
			results.ArrayOfResults = floatResultsArray
		}
		t.initResults(stub,args, modelName, currentData.DataName ,results.ArrayOfResults)
	}
	return shim.Success(nil)
}

func (t *SimpleModel) insertedDataFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	dataName := args[0]
	var results Results
	var payload FilePayload
	var dataJson DataFlex

	//--------------------------------------------------

	// setting the ip address of API
	// setting the ip address of API
	SparkIp :="http://192.168.144.2:8080/"
	MLR3Ip := "http://192.168.144.3:1030/"

	//--------------------------------------------------

	//getting model stored in couchDB-------------------
	dataBytes, err := stub.GetState(dataName)
	if err != nil {
		return shim.Error(err.Error())
	}

	err =json.Unmarshal(dataBytes, &dataJson)
	if err != nil {
		return shim.Error(err.Error())
	}
	payload.Data= dataJson

	//--------------------------------------------------
	//getting data stored in couchDB--------------------
	var wrappedModel[]ModelWrapper
	queryString :="{\"selector\":{\"ObjectType\": \"modelFile\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}"
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(queryResults, &wrappedModel)
	if err != nil {
		return shim.Error(err.Error())
	}

	for i := 0; i < len(wrappedModel); i++ {
		currentModel := wrappedModel[i].Record
		payload.Model = currentModel

		//get validation results for each data---------------
		url := SparkIp + "apiValidate" + currentModel.ModelType
		if currentModel.LibraryType == "MLR3" {
			url = MLR3Ip + "apiValidate"
		}
		payloadJson, err := json.Marshal(payload)

		responseBytes := HttpPost(url, payloadJson)

		if currentModel.LibraryType == "AS" {
			err = json.Unmarshal(responseBytes, &results)
			if err != nil {
				return shim.Error(err.Error())
			}
		}
		if currentModel.LibraryType == "MLR3" {
			stringFromBytes := string(responseBytes)
			cleanString := stringFromBytes
			cleanString = cleanString[1 : len(cleanString)-1]
			s := strings.Split(cleanString, ",")
			floatResultsArray := []float64{}
			for i := 0; i < len(s); i++ {
				if i%2 != 0 {
					currentString := strings.Replace(s[i], "[", "", -1)
					currentString = strings.Replace(currentString, "]", "", -1)
					if n, err := strconv.ParseFloat(currentString, 64); err == nil {
						floatResultsArray = append(floatResultsArray, n)
					}
				}
			}
			results.ArrayOfResults = floatResultsArray
		}
		t.initResults(stub, args, currentModel.Name, dataJson.DataName, results.ArrayOfResults)
	}
	return shim.Success(nil)
}

func (t *SimpleModel) testModelFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	modelFile := args[0]
	modelType := args[1]
	libraryType := args[2]

	var payload FilePayload

	var inputValidationResults ModelValidity
	//getting model stored in couchDB-------------------

	modelJson := &ModelFile{"testModel", "test", modelFile, "none", modelType,libraryType,0,}

	// setting the ip address of API
	SparkIp :="http://192.168.144.2:8080/"
	MLR3Ip := "http://192.168.144.3:1030/"

	//get validation results---------------
	url := SparkIp+"apiTest"+ modelType
	if libraryType == "MLR3"{
		url = MLR3Ip + "apiTest"
	}

	payload.Model = *modelJson
	payloadJson, _ := json.Marshal(payload)

	responseBytes := HttpPost(url,payloadJson)
	if libraryType == "MLR3" {
		stringFromBytes := string(responseBytes)
		cleanString := strings.Replace(stringFromBytes, "\\", "", -1)
		cleanString = cleanString[1 : len(cleanString)-1]
		json.Unmarshal([]byte(cleanString), &inputValidationResults)
	}else{
		json.Unmarshal(responseBytes, &inputValidationResults)
	}

	response := make([]byte, 64)
	binary.BigEndian.PutUint64(response, 0)
	if inputValidationResults.ModelValidity != 0{
		binary.BigEndian.PutUint64(response, 1)
	}
	return shim.Success(response)
}

func (t *SimpleModel) testConnection(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	url:="http://192.168.144.2:8081/apiValidateDT"
	HttpGet(url)
	return shim.Success(nil)
}

//Methods for many Model Testing ----------------------------------------------------------------------------------

func (t *SimpleModel) updateAllModels(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	//var data []DataWrapper
	//var currentModel Model
	var wrappedModel []ModelWrapper
	dataOwner:= args[1] //change this later

	//getting data stored in couchDB --------------------
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"data\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	//json.Unmarshal((queryResults), &data)

	queryString = Sprintf("{\"selector\":{\"ObjectType\": \"model\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	queryResults, err = getQueryResultForQueryString(stub, queryString)
	println(queryResults)
	if err != nil {
		return shim.Error(err.Error())
	}
	json.Unmarshal((queryResults), &wrappedModel)
	//---------------------------------------------------

	for i := 0; i < len(wrappedModel); i++ {
		//currentModel = wrappedModel[i].Record
		//getting model stored in couchDB --------------
		if err != nil {
			return shim.Error(err.Error())
		}

		// -----------------------------------------------
		/*currentArrayOfResults = []float64{};
		for j := 0; j < len(data); j++ {
			//result := calculateLogisticModelResults(data[j], currentModel)
			currentArrayOfResults = append(currentArrayOfResults, result)

		}
		t.initResults(stub,args, currentModel.Name, currentArrayOfResults)*/
	}
	return shim.Success(nil)
}

func (t *SimpleModel) getAllModels(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	queryString :="{\"selector\":{\"ObjectType\": \"modelFile\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}"
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleModel) getAllData(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	queryString :="{\"selector\":{\"ObjectType\": \"dataColumns\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}"
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleModel) getAllResults(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	queryString :="{\"selector\":{\"ObjectType\": \"results\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}"
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleModel) updateAllModelsAPI(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	dataOwner:= args[1] //change this later
	//modelNameBase := "model"
	var results Results
	var payload Payload

	// setting the ip address of API
	ip := "http://192.168.144.0:8081/"

	//getting data stored in couchDB--------------------
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"data\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	dataJson, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	//--------------------------------------------------
	//getting model stored in couchDB--------------------
	queryString = Sprintf("{\"selector\":{\"ObjectType\": \"model\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	modelsJson, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}

	//Getting results from all Models from Oracle service
	url := ip + "apiValidateMany"

	payload.Data = dataJson
	payload.Model = modelsJson

	payloadbytes, err := json.Marshal(payload)
	responseBytes := HttpPost(url,payloadbytes)

	//Parsing json form Oracle results and storing them to blockchain
	err = json.Unmarshal(responseBytes, &results)
	if err != nil {
		return shim.Error(err.Error())
	}
	//t.initManyResults(stub,args,modelNameBase, results.ArrayOfResultsMany)
	return shim.Success(nil)
}

//Methods to read data form Blockchain ------------------------------------------------------------------------

func (t *SimpleModel) queryDataByOwner(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	if len(args) < 1 {
		return shim.Error("Incorrect number of arguments. Expecting 1")
	}

	owner := args[0]

	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"data\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", owner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)

	if err != nil {
		return shim.Error(err.Error())
	}
	return shim.Success(queryResults)
}

func (t *SimpleModel) readModel(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var name, jsonResp string
	var err error

	if len(args) != 1 {
		return shim.Error("Incorrect number of arguments. Expecting name of the marble to query")
	}

	name = args[0]
	valAsbytes, err := stub.GetState(name) //read model from chaincode state
	if err != nil {
		jsonResp = "{\"Error\":\"Failed to get state for " + name + "\"}"
		return shim.Error(jsonResp)
	} else if valAsbytes == nil {
		jsonResp = "{\"Error\":\"Model does not exist: " + name + "\"}"
		return shim.Error(jsonResp)
	}
	return shim.Success(valAsbytes)
}

// ID getting method is performance heavy, bet view creation from chaincode is hard so for the sake of the prototype its implemented by reading all entries and geeting the last one's ID
func (t *SimpleModel) GetModelID(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
 	modelOwner:= args[0]
	responseBytes := make([]byte, 64)
	var wrappedModel []ModelWrapper
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"modelFile\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", modelOwner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(queryResults, &wrappedModel)
	if err != nil {
		return shim.Error(err.Error())
	}else{
		 size := len(wrappedModel)
		 binary.BigEndian.PutUint64(responseBytes, uint64(size))
	}
	return shim.Success(responseBytes)
}

func (t *SimpleModel) GetDataID(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var err error
	dataOwner:= args[0]
	responseBytes := make([]byte, 64)
	var wrappedData[]DataColWrapper
	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"dataColumns\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", dataOwner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)
	if err != nil {
		return shim.Error(err.Error())
	}
	err = json.Unmarshal(queryResults, &wrappedData)
	if err != nil {
		return shim.Error(err.Error())
	}else{
		size := len(wrappedData)
		binary.BigEndian.PutUint64(responseBytes, uint64(size))
	}
	return shim.Success(responseBytes)
}

//Methods to put data into Blockchain state DB -------------------------------------------------------------------------

/*func (t *SimpleModel) initTestData(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	owner := args[0]
	ID, err := strconv.ParseUint(args[1], 10, 64)
	dataColName := args[2]
	objectType := "data"
	currentModelData := &Data{objectType,args[3],args[4],args[5],owner, ID}
	DataJSONasBytes, err := json.Marshal(currentModelData)

	err = stub.PutState(dataColName, DataJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}*/

/*func (t *SimpleModel) initColData(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	var data []DataWrapper
	owner := args[0]
	ID, err := strconv.ParseUint(args[1], 10, 64)
	if err != nil {
		return shim.Error(err.Error())
	}
	var sliceX []string
	var sliceY []string
	var sliceRes []string


	queryString := Sprintf("{\"selector\":{\"ObjectType\": \"data\",\"Owner\": \"%s\"}, \"use_index\": [\"indexOwnerDoc\",\"indexOwner\"]}", owner)
	queryResults, err := getQueryResultForQueryString(stub, queryString)

	json.Unmarshal((queryResults), &data)

	objectType := "dataColumns"

	for i := 0; i < len(data); i++ {

		Xfloat, err := strconv.ParseFloat(data[i].Record.XData, 64)
		if err != nil {
			return shim.Error(err.Error())
		}
		Xrounded := Round(Xfloat,5.0)
		XroundedString := Sprintf("%0.5f", Xrounded)

		Yfloat, err := strconv.ParseFloat(data[i].Record.YData, 64)
		if err != nil {
			return shim.Error(err.Error())
		}
		Yrounded := Round(Yfloat,5.0)
		YroundedString := Sprintf("%0.5f", Yrounded)

		sliceX = append(sliceX, XroundedString)
		sliceY = append(sliceY, YroundedString)
		sliceRes = append(sliceRes, data[i].Record.Class)
	}

	currentModelData := &DataCol{objectType,sliceX,sliceY,sliceRes,owner, "DataCol0", ID}
	DataJSONasBytes, err := json.Marshal(currentModelData)

	err = stub.PutState(args[1], DataJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}*/

func stringToDataMatrix(dataString string) [][]string{
	var dataMatrix [][]string
	dataRow := strings.Split(dataString, ">")
	for _, row := range dataRow {
		var tempRow []string
		dataCell := strings.Split(row, ",")
		for _, cell := range dataCell {
			tempRow = append(tempRow, cell)
		}
		dataMatrix = append(dataMatrix,  tempRow)
	}
	return dataMatrix
}

func (t *SimpleModel) initFlexData(stub shim.ChaincodeStubInterface, args []string) pb.Response {
	batchName := args[0]
	owner := args[1]
	ID, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return shim.Error(err.Error())
	}
	stringData := args[3]
	var data [][]string
	data = stringToDataMatrix(stringData)

	class := strings.Split(args[4], ",")

	objectType := "dataColumns"

	currentModelData := &DataFlex{objectType,data,class,owner, batchName,ID}
	DataJSONasBytes, err := json.Marshal(currentModelData)

	err = stub.PutState(batchName, DataJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

func (t *SimpleModel) initResults(stub shim.ChaincodeStubInterface, args []string, modelName string, dataName string, results []float64) pb.Response {

	//currentResults :=  &ResultsArray{ resIdCounter, results, modelName}
	currentResults :=  &ResultsArray{ "results",0, results, modelName, dataName}
	resultsAsBytes, err := json.Marshal(currentResults)
	if err != nil {
		return shim.Error(err.Error())
	}
	key :=  strconv.FormatInt(resIdCounter, 10)
	finalKey := "results" + key
	err = stub.PutState(finalKey, resultsAsBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	resIdCounter++
	return shim.Success(nil)
}

/*func (t *SimpleModel) initManyResults(stub shim.ChaincodeStubInterface, args []string, modelNameBase string, results [][]float64) pb.Response {
	for i := 0; i < len(results); i++ {
		modelName := modelNameBase + strconv.Itoa(i)
		currentResults := &ResultsArray{resIdCounter, results[i], modelName}
		resultsAsBytes, err := json.Marshal(currentResults)
		key := strconv.FormatInt(resIdCounter, 10)
		finalKey := "results" + key
		err = stub.PutState(finalKey, resultsAsBytes)
		if err != nil {
			return shim.Error(err.Error())
		}
		resIdCounter++
	}
	return shim.Success(nil)
}*/

func (t *SimpleModel) initModel(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	var err error

	//   	    0          1        2
	//   "./model.json", "model1", Vaidotas
	// peer chaincode invoke -C myc -n marbles -c '{"Args":["initModel","model.json","model1","Vaidotas"]}'

	model1, err := strconv.ParseFloat(strings.TrimSpace(args[0]), 64);
	model2, err := strconv.ParseFloat(strings.TrimSpace(args[1]), 64);
	model3, err := strconv.ParseFloat(strings.TrimSpace(args[2]), 64);

	parameters := []float64 { model1, model2, model3 }
	modelName := args[3]
	owner := args[4]

	objectType := "model"
	model := &Model{objectType, modelName, parameters, owner}
	modelJSONasBytes, err := json.Marshal(model)

	//size, err := strconv.Atoi(args[2])

	// ==== Check if model already exists ====
	modelAsBytes, err := stub.GetState(modelName)
	if err != nil {
		return shim.Error("Failed to get model: " + err.Error())
	} else if modelAsBytes != nil {
		return shim.Error("This model already exists: " + modelName)
	}

	err = stub.PutState(modelName, modelJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	// ==== Model saved . Return success ====

	return shim.Success(nil)
}


func (t *SimpleModel) initModelFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	modelName := args[0]
	modelType := args[1]
	libraryType :=  args[2]
	owner := args[3]
	ID, err := strconv.ParseUint(args[4], 10, 64)
	objectType := "modelFile"


		model := &ModelFile{objectType, modelName,args[5],owner, modelType, libraryType,ID}
		modelJSONasBytes, err := json.Marshal(model)

		// ==== Check if model already exists ====
		modelAsBytes, err := stub.GetState(modelName)
		if err != nil {
			return shim.Error("Failed to get model: " + err.Error())
		} else if modelAsBytes != nil {
			return shim.Error("This model already exists: " + modelName)
		}

		err = stub.PutState(modelName, modelJSONasBytes)
		if err != nil {
			return shim.Error(err.Error())
		}
	// ==== Model saved . Return success ====

	return shim.Success(nil)
}

func (t *SimpleModel) initDataFile(stub shim.ChaincodeStubInterface, args []string) pb.Response {

	batchName := args[0]
	owner := args[1]
	ID, err := strconv.ParseUint(args[2], 10, 64)
	if err != nil {
		return shim.Error(err.Error())
	}
	sliceX := strings.Split(args[3], ",")
	sliceY := strings.Split(args[4], ",")
	sliceRes := strings.Split(args[5], ",")

	objectType := "dataColumns"

	currentModelData := &DataCol{objectType,sliceX,sliceY,sliceRes,owner, batchName,ID}
	DataJSONasBytes, err := json.Marshal(currentModelData)

	err = stub.PutState(batchName, DataJSONasBytes)
	if err != nil {
		return shim.Error(err.Error())
	}

	return shim.Success(nil)
}

//Methods for parsing blockchain data -----------------------------------------------------------------------

func getQueryResultForQueryString(stub shim.ChaincodeStubInterface, queryString string) ([]byte, error) {

	resultsIterator, err := stub.GetQueryResult(queryString)
	if err != nil {
		return nil, err
	}
	defer resultsIterator.Close()

	buffer, err := constructQueryResponseFromIterator(resultsIterator)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

func constructQueryResponseFromIterator(resultsIterator shim.StateQueryIteratorInterface) (*bytes.Buffer, error) {
	// buffer is a JSON array containing QueryResults
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

	return &buffer, nil
}

//Non blockchain functions ----------------------------------------------------------------------------------

func calculateLogisticModelResults(XData string,YData string, currentModel Model ) float64{

	var currentX float64
	var currentY float64
	currentX, _=strconv.ParseFloat(XData,64)
	currentY, _=strconv.ParseFloat(YData,64)

	sum:= currentModel.Parameters[0]
	sum += currentX * currentModel.Parameters[1]
	sum += currentY * currentModel.Parameters[2]
	result := 1 / (1 + math.Exp(-sum))

	return result
}

func HttpPost(url string , data []byte) []byte{
	resp, err := http.Post(url,"application/json", bytes.NewBuffer(data))
	if err != nil {
		print(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		print(err)
	}
	return body
}

func  HttpGet(url string) []byte {
	var data []byte
	response, err := http.Get(url)
	if err != nil {
		Printf("The HTTP request failed with error %s\n", err)
	} else {
		data, _ = ioutil.ReadAll(response.Body)
	}
	return data
}

func Round (num float64, decimals float64) float64{
	multipilicator :=  math.Pow(10, decimals)
	roundedNum := math.Round(num*multipilicator)/multipilicator
	return roundedNum
}







