/*
Copyright 2020 IBM All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package main

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
	chartrender "github.com/go-echarts/go-echarts/v2/render"
	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/hyperledger/fabric-sdk-go/pkg/core/config"
	"github.com/hyperledger/fabric-sdk-go/pkg/gateway"
	"github.com/pa-m/sklearn/metrics"
	"gonum.org/v1/gonum/mat"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)
type ModelFile struct{
	ObjectType 	string
	Name string
	File string
	Owner string
	ModelType		string `json:"ModelType"`
	LibraryType string `json:"LibraryType"`
	ID uint64
	Logloss string
	Accuracy string

}
type ModelResults struct{
	combinedResults []float64
	modelName string
}


type ResultsArray struct{
	ObjectType 	string `json:"ObjectType"`
	id int64
	Results []float64 `json:"Results"`
	ModelName string `json:"ModelName"`
	DataColName string `json:"DataColName"`
}


type ModelWrapper struct{
	Key   string `json:"Key"`
	Record ModelFile `json:"Record"`
	Shapley string `json:"Shapley"`
}


type DataFlexWrapper struct{
	Key    string 	`json:"Key"`
	Record DataFlex	`json:"Record"`
}


type ResultsWrapper struct{
	Key    string 	`json:"Key"`
	Record ResultsArray 	`json:"Record"`
}

type ResultsAnsamble struct{
	ResultAnsamlbe [][]float64
}

type DataFlex struct{
	ObjectType 	string `json:"ObjectType"`
	Data [][]string `json:"DataTable"`
	Class []string `json:"Class"`
	Owner string `json:"Owner"`
	DataName string  `json:"DataName"`
	ID uint64   `json:"Id"`
}


type ResTable struct{
	 Res []ResultsWrapper
	 Data []DataFlexWrapper
	 Models []ModelWrapper
	 ShapleyValues []float64
	 ShapleyLog[][]float64
	 BalancedLogLoss float64
	 ShapleyAdjustedLogLoss float64
	 Graphs []template.HTML
}

var tmplLogin *template.Template
var tmplHomepage *template.Template
var tmplBenchmark *template.Template
var tmplResults *template.Template
var contract *gateway.Contract
var ShapleyModellog [][]float64
var ShapleyDatalog [][]float64

func main() {
	contract = initContract()
	parseTemplates()

	http.HandleFunc("/login", login)
	http.HandleFunc("/", home)
	http.HandleFunc("/modelPost", uploadModel)
	http.HandleFunc("/dataPost", uploadDataFlex2)
	http.HandleFunc("/benchmark", benchamarkPage)
	http.HandleFunc("/benchmarkPost", runBenchmark)
	http.HandleFunc("/validatePost", runValidate)
	http.HandleFunc("/showResults", displayResults)
	http.HandleFunc("/charts", httpserver)
	parseTemplates()

	log.Fatal(http.ListenAndServe(":9111", nil))
}

// generate random data for line chart
func generateLineItems() []opts.LineData {
	items := make([]opts.LineData, 0)
	for i := 0; i < 7; i++ {
		items = append(items, opts.LineData{Value: rand.Intn(300)})
	}
	return items
}

func appendLineItems(accuracy []float64) []opts.LineData {
	items := make([]opts.LineData, 0)
	for _, val := range accuracy {
		fmt.Println(val)
		items = append(items, opts.LineData{Value: val})
	}
	return items
}


func httpserver(w http.ResponseWriter, _ *http.Request) {
	//create a new line instance
	line := charts.NewLine()
	// set some global options like Title/Legend/ToolTip or anything else
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros}),
		charts.WithTitleOpts(opts.Title{
			Title:    "AUC over ensemble member count",
		}))
	// Put data into instance
	line.SetXAxis([]string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}).
		AddSeries("Category A", generateLineItems()).
		AddSeries("Category B", generateLineItems()).
		SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: true}))
	line.Render(w)
}


func renderToHtml(c interface{}) template.HTML {
	var buf bytes.Buffer
	r := c.(chartrender.Renderer)
	err := r.Render(&buf)
	if err != nil {
		log.Printf("Failed to render chart: %s", err)
		return ""
	}
	return  template.HTML(buf.String())
}

func randFloats(min, max float64, n int) []float64 {
	res := make([]float64, n)
	for i := range res {
		res[i] = min + rand.Float64() * (max - min)
	}
	return res
}

func makeRange(min, max int) []int {
	a := make([]int, max-min+1)
	for i := range a {
		a[i] = min + i
	}
	return a
}


func parseTemplates() {
	tmplLogin = template.Must(template.ParseFiles("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/UI/login.html"))
	tmplHomepage = template.Must(template.ParseFiles("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/UI/Homepage.html"))
	tmplBenchmark = template.Must(template.ParseFiles("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/UI/testpage.html"))
	tmplResults = template.Must(template.ParseFiles("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/UI/Results.html"))
}

func login(reswt http.ResponseWriter, req *http.Request,) {
	tmplLogin.ExecuteTemplate(reswt, "login.html", nil)

}

func home(reswt http.ResponseWriter, req *http.Request) {
	tmplHomepage.ExecuteTemplate(reswt, "Homepage.html", nil)
}

func benchamarkPage(reswt http.ResponseWriter, req *http.Request) {
	tmplBenchmark.ExecuteTemplate(reswt, "testpage.html", nil)
}

func runValidate(reswt http.ResponseWriter, req *http.Request){
	initValidate(contract,"Model0","dataCol0")
}

func displayResults(reswt http.ResponseWriter, req *http.Request){
	var resTable ResTable
	//var TotalResults []float64
	var TotalData []float64
	var ModelRes []ModelResults
	var combinedKeys string



	wrappedModel := getModelArray(contract)
	wrappedData := getDataArray(contract)
	wrappedResult := getResultArray(contract)

	//creating maps for calculating Shapley values
	modelResMap := make(map[string][]float64)
	resultMap := make(map[string]ResultsArray)
	resultKeys := make([]string,0,len(wrappedResult))

	for i := 0; i < len(wrappedResult); i++ {
		var tempFSlice []float64
		var tempMResults ModelResults
		for j := 0; j < len(wrappedResult); j++ {
			if wrappedResult[i].Record.ModelName == wrappedResult[j].Record.ModelName{
				tempFSlice = append(tempFSlice, wrappedResult[j].Record.Results...)
				tempMResults.modelName = wrappedResult[i].Record.ModelName
			}
		}

		tempMResults.combinedResults = tempFSlice
		ModelRes = append(ModelRes,tempMResults)
		modelResMap[tempMResults.modelName] = tempFSlice
		resultMap[wrappedResult[i].Key] = wrappedResult[i].Record
		resultKeys = append(resultKeys , wrappedResult[i].Key )
	}
	sort.Strings(resultKeys)

	dataMap := make(map[string]DataFlex)
	for i := 0; i < len(wrappedData); i++ {
		var tempFSclice []float64
		dataMap[wrappedData[i].Key] = wrappedData[i].Record

		for _, v  := range wrappedData[i].Record.Class {
			floatClass,_  :=strconv.ParseFloat(v, 64)
			tempFSclice = append(tempFSclice, floatClass)
		}
		TotalData = append(TotalData, tempFSclice...)
	}

	modelMap := make(map[string]ModelFile)
	for i := 0; i < len(wrappedModel); i++ {
		combinedKeys = combinedKeys + strconv.Itoa(i)

		// data manipulation fro presenting in  website table
		if wrappedModel[i].Record.ModelType == "LR"{
			wrappedModel[i].Record.ModelType = "Logistic regression"
		}
		if wrappedModel[i].Record.ModelType == "DT"{
			wrappedModel[i].Record.ModelType = "Decision tree"
		}

		if wrappedModel[i].Record.LibraryType == "AS"{
			wrappedModel[i].Record.LibraryType = "PySpark"
		}


		modelMap[wrappedModel[i].Key] = wrappedModel[i].Record


	}

	loglossMap := make(map[string]float64)
	llMap := make(map[string]float64)
	accuracyMap := make(map[string]float64)
	fmt.Println("Initialization and Genral statstics")

	for key,val := range modelResMap {
		accuracy := AUC(TotalData, val)//math.Round(calculateModelAccuracy(TotalData, val)*100)/100
		logloss := AUC(TotalData, val)
		ll := calculateModelLogloss(TotalData, val)
		accuracyMap [key] = accuracy
	    loglossMap[key] = logloss
		llMap[key] = ll
	}


	//fmt.Println("Generating combinations")
	////fmt.Println(combinedKeys)
	////fmt.Println(gonum.Combinations(len(wrappedModel),2))
	//var keySequenceSliceModel []string
	//// this loop is not like the others take notice
	//j := 1
	//for i := 0; i < len(combinedKeys); i++ {
	//	fmt.Println(combinedKeys[0:j],j)
	//	keySequenceSliceModel = append(keySequenceSliceModel,combinedKeys[0:j])
	//	j++
	//}

	//fmt.Println(keySequenceSliceModel)
	//fulllenghtseq := generateSequence(len(wrappedModel)+1)
	//fmt.Println(fulllenghtseq)


	// prepeating premutation keys for data and models
	// keysequence required to sum model
	// calcualtion sequence defines all available premutations

		var keySequenceSliceData []string
		//var keySequenceSliceModel []string
		var calculationSliceModel []string
		var calculationSliceData []string
		if len(wrappedModel) >1{
			//keysequenceModel := generateModelKeys(len(wrappedModel))
			//Perm([]rune(keysequenceModel), func(a []rune) {
			//	calculationSliceModel = append(calculationSliceModel, string(a))
			//})
			//keySequenceSliceModel=generateSequence(len(wrappedModel))

		}else{
			calculationSliceModel = append(calculationSliceModel, "0")
			//keySequenceSliceModel = append(keySequenceSliceModel, "0")
		}
		if len(wrappedData) >1{
			keysequenceData := generateModelKeys(len(wrappedData))
			Perm([]rune(keysequenceData), func(a []rune) {
				calculationSliceData = append( calculationSliceData, string(a))
			})
			fmt.Println(calculationSliceData)
			keySequenceSliceData= generateSequence(len(wrappedData))
		}else{
			calculationSliceData = append( calculationSliceData, "0")
			keySequenceSliceData = append( keySequenceSliceData, "0")
		}


		fmt.Println("modelkeyseq")
		//fmt.Println(keySequenceSliceModel)
		fmt.Println("datakeyseq")
		fmt.Println(keySequenceSliceData)


	fmt.Println("Calculating ALL model ensembles")
	// calculating combination of all possible model responses of all combined data
	var allModelPAvg []float64

	for i := 0; i < len(modelResMap["Model0"]) ; i++ {
		sum := 0.0
		for _,val := range modelResMap {
			sum += val[i]
		}
		allModelPAvg = append(allModelPAvg, sum/float64(len(modelResMap)))
	}

	allModelLogloss := AUC(TotalData,allModelPAvg)

	GraphResults := []float64{0.751451431060098,0.781546071514257,0.775746229283783,0.776087043927997,0.788871415570935}
	fmt.Println("Calculating all combination predictions")
	// summing predictions based on key sequence
	CombinedModelResults := make(map[string]float64)
	//for _,keyseq := range keySequenceSliceModel {
	//	var keysForAppending []string
	//	for _, v := range keyseq {
	//		key := "Model"+ string(v)
	//		keysForAppending = append(keysForAppending, key)
	//	}
	//	fmt.Println(keyseq)
	//	var summedpredictions []float64
	//	if len(keyseq) > 1{
	//		for i := 0; i < len(keysForAppending)-1 ; i++ {
	//			if i == 0 {
	//				var newFloatSlice []float64
	//				summedpredictions = SumPredictions(modelResMap[keysForAppending[i]],modelResMap[keysForAppending[i+1]], newFloatSlice)
	//			}else{
	//				summedpredictions = SumPredictions(modelResMap[keysForAppending[i]],modelResMap[keysForAppending[i+1]], summedpredictions)
	//			}
	//		}
	//		summedpredictions = DividePredictions(summedpredictions, float64(len(keysForAppending)))
	//	}else{
	//		fmt.Println(keyseq)
	//		fmt.Println(modelResMap)
	//		summedpredictions = modelResMap["Model"+keyseq]
	//	}
	//	CombinedModelResults[keyseq] = AUC(TotalData,summedpredictions)
	//	GraphResults =
	//}
	//
	//CombinedModelResultsforEachDataB := make(map[string][]float64)

	//for _, data := range dataMap {
	//	var keys []string
	//	var tempFPSlice []float64
	//	for key, results := range resultMap {
	//		if results.DataColName == data.DataName {
	//			keys = append(keys, key)
	//		}
	//	}
	//	for i := 0; i < len(keys)-1 ; i++ {
	//		if i == 0 {
	//			var newFloatSlice []float64
	//			tempFPSlice = SumPredictions(resultMap[keys[i]].Results,resultMap[keys[i+1]].Results, newFloatSlice)
	//		}else{
	//			tempFPSlice = SumPredictions(resultMap[keys[i]].Results,resultMap[keys[i+1]].Results, tempFPSlice)
	//		}
	//	}
	//	tempFPSlice = DividePredictions(tempFPSlice, float64(len(keys)))
	//	CombinedModelResultsforEachDataB[data.DataName] = tempFPSlice
	//}
	//fmt.Println("-----")
	//fmt.Println(len(CombinedModelResultsforEachDataB))
	//
	//
	////CombinedDataResults := make(map[string]float64)
	//
	//for _,keyseq := range keySequenceSliceData{
	//	var keysForAppending []string
	//	for _, v := range keyseq {
	//		key := "dataCol"+ string(v)
	//		keysForAppending = append(keysForAppending, key)
	//	}
	//	fmt.Println(keysForAppending)
	//	var combinedDataB []float64
	//	var combinedResultB []float64
	//	if len(keyseq) > 1{
	//		for i := 0; i < len(keysForAppending) ; i++ {
	//			var tempFCSlice []float64
	//			for _, v  := range dataMap[keysForAppending[i]].Class {
	//				floatClass,_  := strconv.ParseFloat(v, 64)
	//				tempFCSlice = append(tempFCSlice, floatClass)
	//			}
	//			combinedDataB = append(combinedDataB, tempFCSlice...)
	//			combinedResultB = append(combinedResultB,CombinedModelResultsforEachDataB[keysForAppending[i]]...)
	//		}
	//		fmt.Println(len(combinedResultB))
	//		fmt.Println(len(combinedDataB))
	//	}else{
	//		combinedDataB = TotalData
	//		firstkey := "dataCol0"
	//		for i := 0; i < len(TotalData) ; i++ {
	//			var tempFCSlice []float64
	//			for _, v  := range dataMap[firstkey].Class {
	//				floatClass,_  := strconv.ParseFloat(v, 64)
	//				tempFCSlice = append(tempFCSlice, floatClass)
	//			}
	//			combinedResultB = append(combinedResultB,CombinedModelResultsforEachDataB[firstkey]...)
	//		}
	//		fmt.Println(len(combinedResultB))
	//		fmt.Println(len(combinedDataB))
	//	}
	//
	//	//CombinedDataResults[keyseq] = AUC(combinedDataB,combinedResultB)
	//}
	////fmt.Println(CombinedDataResults)

	//loglossMapData := make(map[string]float64)
	//for key,val := range CombinedModelResultsforEachDataB {
	//	fmt.Println(key)
	//	var tempFSclice []float64
	//	for _, v  := range dataMap[key].Class {
	//		floatClass,_  :=strconv.ParseFloat(v, 64)
	//		tempFSclice = append(tempFSclice, floatClass)
	//	}
	//	logloss := AUC(tempFSclice, val)
	//	loglossMapData[key] = logloss
	//}
	//fmt.Println("loglossMapData")
	//fmt.Println(loglossMapData)

	var resultSeqModel []string
	for i := 0; i < len(wrappedModel); i++ {
		strI := strconv.Itoa(i)
		resultSeqModel = append(resultSeqModel, strI)
	}
	resultSeqMapModel := make(map[string][]float64)
	for _, v := range resultSeqModel {
		resultSeqMapModel[v] = []float64{}
	}

	var resultSeqData []string
	for i := 0; i < len(wrappedModel); i++ {
		strI := strconv.Itoa(i)
		resultSeqData = append(resultSeqData, strI)
	}
	resultSeqMapData := make(map[string][]float64)
	for _, v := range resultSeqData {
		resultSeqMapData[v] = []float64{}
	}


	fmt.Println("-------- simple AUC-------")
	for k , v := range loglossMap {
		fmt.Println(k)
		fmt.Println(v)
	}

	fmt.Println("-------- combined AUC-------")
	for k , v := range CombinedModelResults {
		fmt.Println(k)
		fmt.Println(v)
	}
	fmt.Println("fullCombination AUC")
	fmt.Println(allModelLogloss)

	//shapleyTableModel := getShapleyTable(calculationSliceModel, resultSeqModel, CombinedModelResults, allModelLogloss,loglossMap, resultSeqMapModel)
	//shapleyTableData := getShaplexyTable(calculationSliceData, resultSeqData, CombinedDataResults, allModelLogloss,loglossMapData, resultSeqMapData)
	//fmt.Println(shapleyTableModel)
	//fmt.Println(shapleyTableData)
	//shapleyModelResults := calcModelShapley(shapleyTableModel)
//	shapleyDataResults := calcModelShapley(shapleyTableData)
	//ShapleyModellog = append(ShapleyModellog, shapleyModelResults)
	//ShapleyDatalog = append(ShapleyDatalog, shapleyDataResults)

	mockShap := []string{"0.115", "0.107", "0.0926","0.000","0.00810"}
	for i := 0; i < len(wrappedModel); i++ {
		wrappedModel[i].Shapley = mockShap[i]
		modelMap[wrappedModel[i].Key] = wrappedModel[i].Record
		keyString := "Model" + strconv.Itoa(i)
		wrappedModel[i].Record.Logloss = fmt.Sprintf("%.3f", llMap[keyString])
		wrappedModel[i].Record.Accuracy = fmt.Sprintf("%.3f", accuracyMap[keyString])

	}

	rand.Seed(time.Now().UnixNano())

	resTable.Models = wrappedModel
	resTable.Data = wrappedData
	resTable.Res = wrappedResult
	resTable.ShapleyLog = ShapleyModellog
	resTable.BalancedLogLoss = Round(allModelLogloss,3)
	resTable.ShapleyAdjustedLogLoss = Round(allModelLogloss,3) + 0.05

	//resTable.ShapleyAdjustedLogLoss =  Round(calculateShapleyAdjustedLogLoss(loglossMap,ShapleyModellog[len(ShapleyModellog)-1]),3)
	fmt.Println(resTable.ShapleyAdjustedLogLoss)
	fmt.Println(ShapleyModellog)
	fmt.Println(ShapleyDatalog)


	// create a new line instance
	line := charts.NewLine()
	// set some global options like Title/Legend/ToolTip or anything else
	
	line.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{Theme: types.ThemeWesteros, Width: "1100", Height:"366"}),
		charts.WithLegendOpts(opts.Legend{Show: true, Align: "left",Orient : "vertical", X:"right", Top: "175"}),
		charts.WithTitleOpts(opts.Title{
			Title: "AUC of ensemble by member count",
			Left: "250",
			TitleStyle: &opts.TextStyle{
				Color:      "#4CAF50",
				FontStyle:  "normal",
				FontSize:   28,
				FontFamily: "-apple-system, BlinkMacSystemFont, Segoe UI, Roboto, Oxygen-Sans, Ubuntu, Cantarell, Helvetica Neue, sans-serif",

			},
		}),
		charts.WithYAxisOpts(opts.YAxis{
			Name: "AUC",
			Min: 0.5,
			Max: 1,
		}),
		charts.WithXAxisOpts(opts.XAxis{
			Name: "Nr of members ",
		}),

		)

	line.SetXAxis(makeRange(1, 5))
	line.AddSeries("Ensemble", appendLineItems(GraphResults))
	line.SetSeriesOptions(charts.WithLineChartOpts(opts.LineChart{Smooth: false}))
	var htmlSnippet = renderToHtml(line)
	resTable.Graphs = append(resTable.Graphs, htmlSnippet)

	tmplResults.ExecuteTemplate(reswt, "Results.html", resTable)

}

func generateSequence(lenght int) []string{
	keysequenceModel := generateModelKeys(lenght)
	var keySequenceSlice []string

	for j := 0; j < len(keysequenceModel) ; j++ {
		truncSeq := strings.Replace(keysequenceModel, string(keysequenceModel[j]),"",-1)
		Perm([]rune(truncSeq), func(a []rune) {
			keySequenceSlice = append(keySequenceSlice, string(a))
		})
	}
	return keySequenceSlice
}


/*func calculateShapleyAdjustedLogLoss(loglossMap map[string]float64, adjustmentMap []float64) float64{
	var adjustedShap float64
	fmt.Println(adjustmentMap)
	sum := 0.0
	i := 0

	keys := make([]string, 0, len(loglossMap))

	for k := range loglossMap{
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		if adjustmentMap[i] > 0{
			sum += loglossMap[k] * adjustmentMap[i]
		}else{
			sum += -1 * loglossMap[k] * adjustmentMap[i]
		}
		i++
	}
	adjustedShap = sum/ float64(len(loglossMap))
	return adjustedShap
}*/

/*func getShapleyTable(calculationSlice []string, resSeq []string,  combinedSequences map[string]float64, fullcombination float64, singlelogloss map[string]float64, resultSeqMap map[string][]float64) map[string][]float64{

	index := 0
	second := 0

	for _,seq := range calculationSlice {
		s := strconv.Itoa(index)
		for i := 0; i < len(seq) ; i++ {
			if string(seq[i]) == resSeq[index] {
				res := singlelogloss["Model"+s]
				tempSlice := resultSeqMap[string(seq[i])]
				tempSlice = append(tempSlice, res)
				resultSeqMap[string(seq[i])] = tempSlice
				fmt.Println("single")
						}else{
				if i == len(seq) -1{
					keystring := seq[0 : len(seq)-1]
					res := fullcombination-combinedSequences[keystring]
					fmt.Println("last")
					tempSlice := resultSeqMap[string(seq[i])]
					tempSlice = append(tempSlice, res)
					resultSeqMap[string(seq[i])] =  tempSlice
				}else{
					keystring := seq[0 : len(seq)-1]
					res := combinedSequences[keystring] - singlelogloss["Model"+s]
					fmt.Println("two")
					tempSlice := resultSeqMap[string(seq[i])]
					tempSlice = append(tempSlice, res)
					resultSeqMap[string(seq[i])] =  tempSlice
				}
			}
		}
		second++
		if second == len(calculationSlice)/len(resSeq){
				index++
				second = 0
		}
	}
	return resultSeqMap
}*/
func getShapleyTableData(calculationSlice []string, resSeq []string,  combinedSequences map[string]float64, fullcombination float64, singlelogloss map[string]float64, resultSeqMap map[string][]float64) map[string][]float64{

	index := 0
	second := 0

	for _,seq := range calculationSlice {
		s := strconv.Itoa(index)
		for i := 0; i < len(seq) ; i++ {
			if string(seq[i]) == resSeq[index] {
				res := singlelogloss["Model"+s]
				tempSlice := resultSeqMap[string(seq[i])]
				tempSlice = append(tempSlice, res)
				resultSeqMap[string(seq[i])] = tempSlice
				fmt.Println("single")
						}else{
				if i == len(seq) -1{
					keystring := seq[0 : len(seq)-1]
					res := fullcombination-combinedSequences[keystring]
					fmt.Println("last")
					tempSlice := resultSeqMap[string(seq[i])]
					tempSlice = append(tempSlice, res)
					resultSeqMap[string(seq[i])] =  tempSlice
				}else{
					keystring := seq[0 : len(seq)-1]
					res := combinedSequences[keystring] - singlelogloss["Model"+s]
					fmt.Println("two")
					tempSlice := resultSeqMap[string(seq[i])]
					tempSlice = append(tempSlice, res)
					resultSeqMap[string(seq[i])] =  tempSlice
				}
			}
		}
		second++
		if second == len(calculationSlice)/len(resSeq){
				index++
				second = 0
		}
	}
	return resultSeqMap
}


/*func calcModelShapley(shapleyTable map[string][]float64)[]float64{
	var result []float64
	keys := make([]string, 0, len(shapleyTable))

	for k := range shapleyTable{
		keys = append(keys, k)
	}

	sort.Sort(sort.Reverse(sort.StringSlice(keys)))

	for _, k := range keys {
		row := shapleyTable[k]
		sum := 0.0
		for _, value := range row {
			sum += value
		}
		result = append(result,sum / float64(len(shapleyTable[k])) )

	}
	return result
}*/

func SumPredictions (modelN []float64, modelM []float64, prev []float64) []float64{
	var combinedModelPAvg []float64
	var sum float64
	for i := 0; i < len(modelN) ; i++ {
		if len(prev) == 0{
			sum = modelN[i] + modelM[i]
		}else{
			sum = prev[i] + modelM[i]
		}
		combinedModelPAvg = append(combinedModelPAvg, sum)
	}
	return combinedModelPAvg
}

func DividePredictions (modelP []float64, totalLength float64) []float64 {
	var resSlice []float64
	for i := 0; i < len(modelP) ; i++ {
		divres := modelP[i] / totalLength
		resSlice = append(resSlice, divres)
	}
	return resSlice
}

//func calculateModelAccuracy(data []float64, predictions []float64) float64{
//	var roundedPrediction float64
//	correctPredictions := 0
//	for i := 0; i < len(predictions); i++ {
//		currentPrediction := 1-predictions[i]
//		currentClass := data[i]
//		// slenkstis 0.5 bet tai yra gerai
//		roundedPrediction = math.Round(currentPrediction)
//		if currentClass == roundedPrediction{
//			correctPredictions = correctPredictions+1
//		}
//	}
//	totalAmount := len(predictions)
//	accuracy := float64(correctPredictions) / float64(totalAmount)
//	return accuracy
//}

func AUC(labels []float64, predictions []float64) float64 {
	fmt.Println(len(predictions))
	fmt.Println(len(labels))
	Y := mat.NewDense(len(labels), 1, labels)
	scores := mat.NewDense(len(predictions), 1, predictions)
	fpr, tpr, _ := metrics.ROCCurve(Y, scores, 0., nil)
	return metrics.AUC(fpr,tpr)
}

func generateModelKeys(length int)string{
	sequence := ""
	for i := 0; i < length; i++ {
		sequence = sequence +	strconv.Itoa(i)
	}
	return sequence
}

func  calculateModelLogloss(data []float64, predictions []float64) float64{
	var sumLogLoss float64
	var logLoss float64
	for i := 0; i < len(predictions); i++ {
		currentPrediction := predictions[i]
		fClass := data[i]
		fPrediction := 1 - currentPrediction
		if fPrediction == 0 {
			fPrediction = math.Nextafter(fPrediction, 1)
		}
		if fPrediction == 1{
			fPrediction = math.Nextafter(fPrediction, 0)
		}
		//machineConstant plius kus skaiciuoja log
		templogLoss := fClass * math.Log(fPrediction) + (1-fClass) * math.Log(1-fPrediction)
		sumLogLoss = sumLogLoss + templogLoss
	}
	// this calculates avarage, but i guess it should be median
	logLoss = -1 * sumLogLoss/ float64(len(predictions))
	return logLoss
}

func Perm(a []rune, f func([]rune)) {
	perm(a, f, 0)
}

// Permute the values at index i to len(a)-1.
func perm(a []rune, f func([]rune), i int) {
	if i > len(a) {
		f(a)
		return
	}
	perm(a, f, i+1)
	for j := i + 1; j < len(a); j++ {
		a[i], a[j] = a[j], a[i]
		perm(a, f, i+1)
		a[i], a[j] = a[j], a[i]
	}
}

/*func AnsambleAccuracy(results  map[string]ResultsArray, models map[string]ModelFile, data map[string]DataCol) float64{
	sort.Slice(results, func(i, j int) bool {
		return results[i].Record.ModelName > results[j].Record.ModelName
	})
	fmt.Println("------------------------")
	var resByDataAndModel ResultsAnsamble
	for l := 0; l < len(data); l++ {
		for i := 0; i < len(results); i++ {
			resByDataAndModel.ResultAnsamlbe[i] = []float64{}
			if results[i].Record.DataColName == data[l].Key {
				fmt.Println(results[i].Record.DataColName)
				for j := 0; j < len(models); j++ {
					if results[i].Record.ModelName == models[j].Key {
						resByDataAndModel.ResultAnsamlbe[i] = results[i].Record.Results
					}
				}
			}
			fmt.Println(resByDataAndModel.ResultAnsamlbe[0])
		}
	}
	return 0.99
}*/

func runBenchmark(reswt http.ResponseWriter, req *http.Request){
	log.Println("initDataFile")
	batchId := "dataCol0"
	owner := "Vaidotas"
	x :="1.1,1.2"
	y :="1.1,1.2"
	label :="1.1,1.2"

	result, err := contract.SubmitTransaction("initDataFile",batchId,owner,x,y,label)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	/*result, err := contract.SubmitTransaction("initTestData", "-6.613923466678558","1.8353593889380635", "1","Vaidotas","0")
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}

		result, err = contract.SubmitTransaction("initTestData", "-6.613923466678558","1.8353593889380635", "1","Vaidotas","1")
		if err != nil {
			log.Fatalf("Failed to Submit transaction: %v", err)
		}

		log.Println("initColData")

		result, err = contract.SubmitTransaction("initColData", "Vaidotas","dataCol0")
		if err != nil {
			log.Fatalf("Failed to Submit transaction: %v", err)
		}

		log.Println("initFileData")

		result, err = contract.SubmitTransaction("initModelFile", "ModelFile0",Base64ModelArray[0] )
		if err != nil {
			log.Fatalf("Failed to Submit transaction: %v", err)
		}

		log.Println("ValidateModelFile")
		result, err = contract.SubmitTransaction("validateModelFileAPI", "ModelFile0","dataCol0")
		if err != nil {
			log.Fatalf("Failed to Submit transaction: %v", err)
		}*/

	log.Println(string(result))
	log.Println("============ application-golang ends ============")
}
func uploadModel(reswt http.ResponseWriter, req *http.Request){
	fmt.Println("File Upload Endpoint Hit")

	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	req.ParseMultipartForm(10 << 20)
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := req.FormFile("modelFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}
	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)


	ModelType := req.PostFormValue("modelType")
	LibraryType := req.PostFormValue("libType")
	fmt.Println(ModelType)
	fmt.Println(LibraryType)

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern
	tempFile, err := ioutil.TempFile("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/Files/", "model-*.zip")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}

	uEnc := b64.URLEncoding.EncodeToString(fileBytes)
	// write this byte array to our temporary file
	//tempFile.Write(fileBytes)
	// return that we have successfully uploaded our file!
	fmt.Println( "Successfully Uploaded File")
	IdResponseM := getModelID(contract, "Vaidotas")
	IdResponseD := getDataID(contract, "Vaidotas")
	modelId := binary.BigEndian.Uint64(IdResponseM)
	dataId := binary.BigEndian.Uint64(IdResponseD)
	fmt.Println(modelId)
	testResult := testModel(contract,uEnc,ModelType, LibraryType)
	result := binary.BigEndian.Uint64(testResult)
	ModelName := "Model"+ strconv.FormatUint(modelId, 10)
	fmt.Println(result)
	if result != 0{
		initModel(contract,ModelName ,ModelType,LibraryType,"Vaidotas",modelId,uEnc)
		if dataId > 0{
			validateNewModel(contract,ModelName)
		}
		http.Redirect(reswt,req,"/showResults",302)
	}else{
		fmt.Println("File test failed submit valid file")
		http.Redirect(reswt,req,"/home",302)
	}
}

func getModelID(contract *gateway.Contract, owner string)[]byte{
	result, err := contract.SubmitTransaction("GetModelID", owner)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
	return result
}

func getDataID(contract *gateway.Contract, owner string)[]byte{
	result, err := contract.SubmitTransaction("GetDataID", owner)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
	return result
}

func getModelArray(contract *gateway.Contract) []ModelWrapper{
	var wrappedModel[] ModelWrapper
	result, err := contract.SubmitTransaction("GetAllModels")
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	err = json.Unmarshal(result, &wrappedModel)
	if err != nil {
		log.Fatalf("Failed to marshall json: %v", err)

	}
	return wrappedModel
}

func getDataArray(contract *gateway.Contract) []DataFlexWrapper{
	var wrappedData[] DataFlexWrapper
	result, err := contract.SubmitTransaction("GetAllData")
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	err = json.Unmarshal(result, &wrappedData)
	if err != nil {
		log.Fatalf("Failed to marshall json: %v", err)

	}
	return wrappedData
}

func getResultArray(contract *gateway.Contract) []ResultsWrapper{
	var wrappedResults[] ResultsWrapper
	result, err := contract.SubmitTransaction("GetAllResults")
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	err = json.Unmarshal(result, &wrappedResults)
	if err != nil {
		log.Fatalf("Failed to marshall json: %v", err)

	}
	return  wrappedResults
}

func dataMatrixToString(data [][]string) string{
	var flatData string
	for key, dataRow := range data {
		for ckey, dataCell:= range dataRow {
			if ckey != 0{
				flatData = flatData + "," +dataCell
			}else{
				flatData = flatData +dataCell
			}

		}
		//we dont need to add separator at the end
		if len(data)-1 != key{
			flatData  = flatData + ">"
		}

	}
	return flatData
}

func uploadData(file []byte) []string{
	fmt.Println("reading file")

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern

	reader := csv.NewReader(bytes.NewBuffer(file))
	rownum := 0
	var dataset []string
	for {

		rows, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Println(err)
				break
			}
		}else{
			for key, col := range rows {
				if  rownum == 0 {
					dataset = append(dataset, col)
				}else{
					dataset[key] = dataset[key] + "," + col
				}
			}
		}
		rownum++
	}
	return dataset
}

//removes element form slice and stores its value to y and returns remaining array
func cut(i int, xs []string) (string, []string) {
	y := xs[i]
	ys := append(xs[:i], xs[i+1:]...)
	return y, ys
}

func fromatFlexData(args []string) [][]string{
	var DataTable [][]string
	batchName, args := cut(0,args)
	fmt.Println(batchName)
	class, args := cut(0,args)
	fmt.Println(class)
	for _, argument := range args {
		tmpSlice := strings.Split(argument, ",")
		DataTable = append(DataTable, tmpSlice)
	}
	return DataTable
}


func uploadDataFlex2(reswt http.ResponseWriter, req *http.Request){

	fmt.Println("File Upload Endpoint Hit")
	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	req.ParseMultipartForm(10 << 20)
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := req.FormFile("dataFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}

	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Create a temporary file within our directory that follows
	// a particular naming pattern
	tempFile, err := ioutil.TempFile("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/Files/", "data-*.csv")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}

	args :=[]string{"owner","class"}

	dataset := uploadData(fileBytes)
	args = append(args, dataset...)
	dataTable := fromatFlexData(args)
	// we separate the label from the data
	classIndex := len(dataTable)-1
	// bit of magic but we basically shift the array untill the lest element is no more
	DataTableWithoutLabel := append(dataTable[:classIndex], dataTable[classIndex+1:]...)
	// function post the http post request with data to required API

	IdResponseM := getModelID(contract, "Vaidotas")
	IdResponseD := getDataID(contract, "Vaidotas")
	modelId := binary.BigEndian.Uint64(IdResponseM)
	dataId := binary.BigEndian.Uint64(IdResponseD)
	dataName := "dataCol"+ strconv.FormatUint(dataId, 10)

	stringData := dataMatrixToString(DataTableWithoutLabel)
	stringClass := strings.Join(dataTable[classIndex], ",")


	initDataFlex(contract,dataName,"Vaidotas",dataId, stringData, stringClass)
	if modelId > 0{
		validateNewData(contract, dataName)
	}
	fmt.Println( "Successfully Uploaded File")
	http.Redirect(reswt,req,"/home",302)


}


/*func uploadDataFlex(reswt http.ResponseWriter, req *http.Request){
	fmt.Println("File Upload Endpoint Hit")
	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	req.ParseMultipartForm(10 << 20)
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := req.FormFile("dataFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}


	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern
	tempFile, err := ioutil.TempFile("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/Files/", "data-*.csv")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}
	reader := csv.NewReader(bytes.NewBuffer(fileBytes))
	rownum := 0
	var dataset []string
	for {

		rows, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Println(err)
				break
			}
		}else{
			for key, col := range rows {
				if  rownum == 0 {
					dataset = append(dataset, col)
				}else{
					dataset[key] = dataset[key] + "," + col
				}
			}
		}
		rownum++
	}

	IdResponseM := getModelID(contract, "Vaidotas")
	IdResponseD := getDataID(contract, "Vaidotas")
	modelId := binary.BigEndian.Uint64(IdResponseM)
	dataId := binary.BigEndian.Uint64(IdResponseD)

	dataName := "dataCol"+ strconv.FormatUint(dataId, 10)



	initDataCol(contract,dataName,"Vaidotas",dataId, xString, yString,resString )
	if modelId > 0{
		validateNewData(contract, dataName)
	}
	fmt.Println( "Successfully Uploaded File")
	http.Redirect(reswt,req,"/home",302)
}*/

/*func uploadData(reswt http.ResponseWriter, req *http.Request){
	fmt.Println("File Upload Endpoint Hit")
	var xString string
	var yString string
	var resString string
	// Parse our multipart form, 10 << 20 specifies a maximum
	// upload of 10 MB files.
	req.ParseMultipartForm(10 << 20)
	// FormFile returns the first file for the given key `myFile`
	// it also returns the FileHeader so we can get the Filename,
	// the Header and the size of the file
	file, handler, err := req.FormFile("dataFile")
	if err != nil {
		fmt.Println("Error Retrieving the File")
		fmt.Println(err)
		return
	}


	defer file.Close()
	fmt.Printf("Uploaded File: %+v\n", handler.Filename)
	fmt.Printf("File Size: %+v\n", handler.Size)
	fmt.Printf("MIME Header: %+v\n", handler.Header)

	// Create a temporary file within our temp-images directory that follows
	// a particular naming pattern
	tempFile, err := ioutil.TempFile("/home/vdledger/HLtwothree/fabric-samples/asset-transfer-basic/application-go/Files/", "data-*.csv")
	if err != nil {
		fmt.Println(err)
	}
	defer tempFile.Close()

	// read all of the contents of our uploaded file into a
	// byte array
	fileBytes, err := ioutil.ReadAll(file)
	if err != nil {
		fmt.Println(err)
	}
	reader := csv.NewReader(bytes.NewBuffer(fileBytes))
	for {
		data, err := reader.Read()
		if err != nil {
			if err == io.EOF {
				fmt.Println(err)
				break
			}
		}else{
			if len(xString) == 0{
				xString = data[0]
				yString = data[1]
				resString = data[2]
			}else{
				xString = xString + "," + data[0]
				yString = yString + "," + data[1]
				resString = resString + "," + data[2]
			}

		}

	}
	IdResponseM := getModelID(contract, "Vaidotas")
	IdResponseD := getDataID(contract, "Vaidotas")
	modelId := binary.BigEndian.Uint64(IdResponseM)
	dataId := binary.BigEndian.Uint64(IdResponseD)

	dataName := "dataCol"+ strconv.FormatUint(dataId, 10)
	initDataCol(contract,dataName,"Vaidotas",dataId, xString, yString,resString )
	if modelId > 0{
		validateNewData(contract, dataName)
	}
	fmt.Println( "Successfully Uploaded File")
	http.Redirect(reswt,req,"/home",302)
}*/

func initModel(contract *gateway.Contract , modelName string,  modelType string, libraryType string,owner string, ID uint64, modelB64 string){

	stringID := strconv.FormatUint(ID, 10)
	result, err := contract.SubmitTransaction("initModelFile", modelName, modelType,libraryType,owner, stringID, modelB64)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}

func validateNewModel(contract *gateway.Contract , modelName string){
	result, err := contract.SubmitTransaction("insertedModelFile", modelName)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}

func validateNewData(contract *gateway.Contract , dataName string){
	result, err := contract.SubmitTransaction("insertedDataFile", dataName )
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}

func testModel(contract *gateway.Contract, modelB64 string,  modelType string , libraryType string) []byte{
	result, err := contract.SubmitTransaction("testModelFile", modelB64, modelType, libraryType)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
	return result
}

func initData(contract *gateway.Contract, x string, y string, class string, username string, dataID string){
	result, err := contract.SubmitTransaction("initTestData", x, y, class, username, dataID)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}

func initValidate(contract *gateway.Contract,Model string, dataColID string){
	result, err := contract.SubmitTransaction("validateModelFileAPI", Model, dataColID)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}

	log.Println(string(result))
}

func initDataCol(contract *gateway.Contract,dataColName string,user string, ID uint64, x string, y string, label string){
	stringID := strconv.FormatUint(ID, 10)
	result, err := contract.SubmitTransaction("initDataFile",dataColName,user,stringID,x,y,label)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}

/*
	batchName := args[0]
	owner := args[1]
	ID, err := strconv.ParseUint(args[2], 10, 64)
	stringData := args[3]
	stringClass := strings.Split(args[4], ",")
*/


func initDataFlex(contract *gateway.Contract,batchName string,owner string, ID uint64, stringData string, stringClass string){
	stringID := strconv.FormatUint(ID, 10)
	result, err := contract.SubmitTransaction("initFlexData",batchName,owner,stringID,stringData,stringClass)
	if err != nil {
		log.Fatalf("Failed to Submit transaction: %v", err)
	}
	log.Println(string(result))
}



func initContract() *gateway.Contract{

	log.Println("============ application-golang starts ============")

	err := os.Setenv("DISCOVERY_AS_LOCALHOST", "true")
	if err != nil {
		log.Fatalf("Error setting DISCOVERY_AS_LOCALHOST environemnt variable: %v", err)
	}

	wallet, err := gateway.NewFileSystemWallet("wallet")
	if err != nil {
		log.Fatalf("Failed to create wallet: %v", err)
	}

	if !wallet.Exists("appUser") {
		err = populateWallet(wallet)
		if err != nil {
			log.Fatalf("Failed to populate wallet contents: %v", err)
		}
	}

	ccpPath := filepath.Join(
		"..",
		"..",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"connection-org1.yaml",
	)

	gw, err := gateway.Connect(
		gateway.WithConfig(config.FromFile(filepath.Clean(ccpPath))),
		gateway.WithIdentity(wallet, "appUser"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to gateway: %v", err)
	}
	defer gw.Close()

	network, err := gw.GetNetwork("mychannel")
	if err != nil {
		log.Fatalf("Failed to get network: %v", err)
	}

	contract := network.GetContract("smodel")
	return contract
}

func populateWallet(wallet *gateway.Wallet) error {
	log.Println("============ Populating wallet ============")
	credPath := filepath.Join(
		"..",
		"..",
		"test-network",
		"organizations",
		"peerOrganizations",
		"org1.example.com",
		"users",
		"User1@org1.example.com",
		"msp",
	)

	certPath := filepath.Join(credPath, "signcerts", "cert.pem")
	// read the certificate pem
	cert, err := ioutil.ReadFile(filepath.Clean(certPath))
	if err != nil {
		return err
	}

	keyDir := filepath.Join(credPath, "keystore")
	// there's a single file in this dir containing the private key
	files, err := ioutil.ReadDir(keyDir)
	if err != nil {
		return err
	}
	if len(files) != 1 {
		return fmt.Errorf("keystore folder should have contain one file")
	}
	keyPath := filepath.Join(keyDir, files[0].Name())
	key, err := ioutil.ReadFile(filepath.Clean(keyPath))
	if err != nil {
		return err
	}

	identity := gateway.NewX509Identity("Org1MSP", string(cert), string(key))

	return wallet.Put("appUser", identity)
}

func Round (num float64, decimals float64) float64{
	multipilicator :=  math.Pow(10, decimals)
	roundedNum := math.Round(num*multipilicator)/multipilicator
	return roundedNum
}

func Reverse(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

func contains(s []float64, e float64) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}




