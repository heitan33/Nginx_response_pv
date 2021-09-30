package main

import (
	"fmt"
	"net/http"
	_"net/http/pprof"
//	"reflect"
	"context"
	"sync"
	"strings"
	"os"
	"bufio"
	"io"
	"encoding/json"
	"strconv"
	"time"
    "gopkg.in/yaml.v2"
	"io/ioutil"
	"github.com/hpcloud/tail"

	"sort"
)


type conf struct {
	VisitUrl  	string 		`yaml:"visitUrl"`
	ResponUrl	string		`yaml:"responUrl"`	
}


func (c *conf) getConf() *conf {
	yamlFile ,err := ioutil.ReadFile("responsTime.yaml")
	if err != nil {
		fmt.Println("yamlFile.Get err", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Println("Unmarshal: ", err.Error())
	}
	return c
}


func Post(data ,visitUrl string) {
	jsoninfo := strings.NewReader(data)
	client := &http.Client{}
	req, err := http.NewRequest("POST", visitUrl, jsoninfo)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", "monitoring_6496d6c7422146fab147ca11d61c19bd")
	resp, err := client.Do(req)
	defer func() {
		if err := recover(); err != nil {
			fmt.Println(err)
			return
	}
		fmt.Println("Process panic done Post")
	}()
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(body))
	fmt.Println(resp.StatusCode)
}

var properties = make(map[string]string)

func init() {
	srcFile, err := os.OpenFile("./responsTime.properties", os.O_RDONLY, 0666)
	num++
	defer srcFile.Close()
	if err != nil {
		fmt.Println("The file not exits.", err)
	} else {
		srcReader := bufio.NewReader(srcFile)
		for {
			str, err := srcReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
			}
			if len(strings.TrimSpace(str)) == 0 || str == "\n" {
				continue
			} else {
				fmt.Println(str)
//				num++
				properties[strings.Replace(strings.Split(str, ":")[0], " ", "", -1)] = strings.Replace(strings.Split(str, ":")[1], " ", "", -1)
			}
		}
	}

	visitSrcFile, err := os.OpenFile("./visitVolume.properties", os.O_RDONLY, 0666)
	num++
	defer visitSrcFile.Close()
	if err != nil {
		fmt.Println("The file not exits.")
	} else {
		srcReader := bufio.NewReader(visitSrcFile)
		for {
			str, err := srcReader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
			}
			if len(strings.TrimSpace(str)) == 0 || str == "\n" {
				continue
			} else {
				fmt.Println(str)
//				num++
				properties[strings.Replace(strings.Split(str, ":")[0], " ", "", -1)] = strings.Replace(strings.Split(str, ":")[1], " ", "", -1)
			}
		}
	}
	return
}


type postData struct {
	ConfigId 		string 		`json:"configId"`
	MinResponseTime int64		`json:"minResponseTime"`
	MaxResponseTime	int64		`json:"maxResponseTime"`
	AvgResponseTime	int64		`json:"avgResponseTime"`
	ProcessType		string 		`json:"processType"`
}

type visitVolume struct {
	ConfigId 		string 		`json:"ConfigId"`
	Volume 			int64 		`json:"volume"`
}

var wg sync.WaitGroup
var num int = 0

type FloatSlice []float64
func (s FloatSlice) Len() int { return len(s) }
func (s FloatSlice) Swap(i, j int){ s[i], s[j] = s[j], s[i] }
func (s FloatSlice) Less(i, j int) bool { return s[i] < s[j] }

func main() {
	var config conf
	var machineId ,logAbsPath string
	var responseTimeList []float64
	urlConfig := config.getConf()
	go http.ListenAndServe(":60000", nil)
	for {
//		c := make(chan string, 1)
		wg.Add(num)
		fmt.Println(properties)
		for machineId ,logAbsPath = range properties {
			machineId = strings.TrimSpace(strings.Replace(machineId, "\n", "" ,-1))
			logAbsPath = strings.TrimSpace(strings.Replace(logAbsPath, "\n", "" ,-1))
			go tailLog(logAbsPath ,machineId ,urlConfig ,&responseTimeList)
		}
		wg.Wait()
//		c <- "stop"
//		close(c)
		responseTimeList = []float64{}
		fmt.Println("本次结束")
	}
}

type PostParameter struct {
	LogAbsPath 	string
	MachineId	string
	UrlConfig 	*conf
}	

func tailLog(logAbsPath ,machineId string, urlConfig *conf, responseTimeList *[]float64) {
	var count int64
	var lock sync.Mutex
	var responseTime string
	var processType string = "HTTP_SERVER" 
	var pare PostParameter

	ctx, cancel := context.WithTimeout(context.Background(), 60 * time.Second)
	defer cancel()
	go func(ctx context.Context) {
		config := tail.Config {
			ReOpen:    true,                                 // 重新打开
			Follow:    true,                                 // 是否跟随
			Location:  &tail.SeekInfo{Offset: 0, Whence: 2}, // 从文件的哪个地方开始读
			MustExist: false,                                // 文件不存在不报错
			Poll:      true,
		}

		tails, err := tail.TailFile(logAbsPath, config)
		if err != nil {
			fmt.Println("tail file failed, err:", err)
			return
		}

		var (
			line *tail.Line
			ok   bool
		)

		for {
			line, ok = <-tails.Lines
			lock.Lock()
			count++
			lock.Unlock()
			if !ok {
				fmt.Printf("tail file close reopen, filename:%s\n", tails.Filename)
				continue
			}
			lineDone := strings.TrimSpace(line.Text)
			responseTime = strings.Split(lineDone ," ")[len(strings.Split(lineDone ," "))-1]
			responsTimeFloat64 ,_ := strconv.ParseFloat(responseTime ,64)
			if responsTimeFloat64 != 0 {
				lock.Lock()	
				*responseTimeList = append(*responseTimeList ,responsTimeFloat64)
				lock.Unlock()
			}
			if !ok {
				fmt.Printf("tail file close reopen, filename:%s\n", tails.Filename)
				continue
			}
		}
	} (ctx)

    pare.LogAbsPath = logAbsPath
    pare.MachineId = machineId
    pare.UrlConfig = urlConfig
	select {
	case <- ctx.Done():
		pare.analysisAndPost(responseTimeList ,count ,processType)

    case <-time.After(time.Duration(time.Second * 62)):
		pare.analysisAndPost(responseTimeList ,count ,processType)
	}
}

func (args PostParameter) analysisAndPost(responseTimeList *[]float64 ,count int64 ,processType string) {
    var data postData
    var visitVolumeData visitVolume
    var minResponseTime, avgResponseTime, maxResponseTime int64
	if strings.Contains(args.LogAbsPath, "error") == false {
		fmt.Println("--------------------------")
		fmt.Println(len(*responseTimeList))
		sort.Sort(FloatSlice(*responseTimeList))
		if len(*responseTimeList) == 0 {
			*responseTimeList = append(*responseTimeList ,0)
		}
		minResponseTime = int64((*responseTimeList)[0] * 1000)
		maxResponseTime = int64((*responseTimeList)[len(*responseTimeList)-1] * 1000)
		sum := 0.0
		for _, add := range *responseTimeList {
		    sum = sum + add
		}
		avgRes := float32(float32(sum) / float32(len(*responseTimeList)))
		avgResponseTimeFloat ,_ := strconv.ParseFloat(fmt.Sprintf("%.3f" ,avgRes), 64)
		avgResponseTime = int64(avgResponseTimeFloat * 1000)
		fmt.Println(avgResponseTime)
		data = postData{ConfigId: args.MachineId, ProcessType: processType, MinResponseTime: minResponseTime, MaxResponseTime: maxResponseTime, AvgResponseTime: avgResponseTime}
		dataJson, err := json.Marshal(data)
		if err != nil {
		    fmt.Println("data json trans err", err.Error())
		}
		dataJsonStr := string(dataJson)
		fmt.Println(dataJsonStr)
		Post(dataJsonStr ,args.UrlConfig.ResponUrl)
	}

	visitVolumeData = visitVolume{ConfigId: args.MachineId, Volume: count}
	visitVolumeDataJson, err := json.Marshal(visitVolumeData)
	if err != nil {
		fmt.Println("data json trans err", err.Error())
	}
	visitVolumeDataJsonStr := string(visitVolumeDataJson)
	fmt.Println(visitVolumeDataJsonStr)
	Post(visitVolumeDataJsonStr ,args.UrlConfig.VisitUrl)
	wg.Done()
}