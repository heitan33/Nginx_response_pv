package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"gopkg.in/yaml.v2"
)

type conf struct {
	VisitUrl  string `yaml:"visitUrl"`
	ResponUrl string `yaml:"responUrl"`
}

func (c *conf) getConf() *conf {
	yamlFile, err := ioutil.ReadFile("responsTime.yaml")
	if err != nil {
		fmt.Println("yamlFile.Get err", err.Error())
	}
	err = yaml.Unmarshal(yamlFile, c)
	if err != nil {
		fmt.Println("Unmarshal: ", err.Error())
	}
	return c
}

func Post(data, visitUrl string) {
	jsoninfo := strings.NewReader(data)
	client := &http.Client{}
	req, err := http.NewRequest("POST", visitUrl, jsoninfo)
	if err != nil {
		fmt.Println(err)
	}
	req.Header.Set("Content-type", "application/json")
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
	//	num++
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
				properties[strings.Replace(strings.Split(str, ":")[0], " ", "", -1)] = strings.Replace(strings.Split(str, ":")[1], " ", "", -1)
			}
		}
	}

	visitSrcFile, err := os.OpenFile("./visitVolume.properties", os.O_RDONLY, 0666)
	//	num++
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
				properties[strings.Replace(strings.Split(str, ":")[0], " ", "", -1)] = strings.Replace(strings.Split(str, ":")[1], " ", "", -1)
			}
		}
	}
	return
}

type PostParameter struct {
	LogAbsPath string
	MachineId  string
	UrlConfig  *conf
}

var wg sync.WaitGroup

type FloatSlice []float64

func (s FloatSlice) Len() int           { return len(s) }
func (s FloatSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s FloatSlice) Less(i, j int) bool { return s[i] < s[j] }

var num int = 0

type postData struct {
	ConfigId        string `json:"configId"`
	MinResponseTime int64  `json:"minResponseTime"`
	MaxResponseTime int64  `json:"maxResponseTime"`
	AvgResponseTime int64  `json:"avgResponseTime"`
	ProcessType     string `json:"processType"`
}

type visitVolume struct {
	ConfigId string `json:"ConfigId"`
	Volume   int64  `json:"volume"`
}

func (args PostParameter) AnalysisAndPost(postResponseTimeList []float64, count int64, processType string) {
	var data postData
	var visitVolumeData visitVolume
	var minResponseTime, avgResponseTime, maxResponseTime int64
	if strings.Contains(args.LogAbsPath, "error") == false {
		fmt.Println("--------------------------")
		fmt.Println(len(postResponseTimeList))
		minResponseTime = int64((postResponseTimeList)[0] * 1000)
		maxResponseTime = int64((postResponseTimeList)[len(postResponseTimeList)-1] * 1000)
		sum := 0.0
		for _, add := range postResponseTimeList {
			sum = sum + add
		}

		avgRes := float32(float32(sum) / float32(len(postResponseTimeList)))
		avgResponseTimeFloat, _ := strconv.ParseFloat(fmt.Sprintf("%.3f", avgRes), 64)
		avgResponseTime = int64(avgResponseTimeFloat * 1000)
		fmt.Println(avgResponseTime)
		data = postData{ConfigId: args.MachineId, ProcessType: processType, MinResponseTime: minResponseTime, MaxResponseTime: maxResponseTime, AvgResponseTime: avgResponseTime}
		dataJson, err := json.Marshal(data)
		if err != nil {
			fmt.Println("data json trans err", err.Error())
		}
		dataJsonStr := string(dataJson)
		fmt.Println(dataJsonStr)
		Post(dataJsonStr, args.UrlConfig.ResponUrl)
	}

	visitVolumeData = visitVolume{ConfigId: args.MachineId, Volume: count}
	visitVolumeDataJson, err := json.Marshal(visitVolumeData)
	if err != nil {
		fmt.Println("data json trans err", err.Error())
	}
	visitVolumeDataJsonStr := string(visitVolumeDataJson)
	fmt.Println(visitVolumeDataJsonStr)
	Post(visitVolumeDataJsonStr, args.UrlConfig.VisitUrl)
	fmt.Println("协程等待退出！")
	return
}

func main() {
	var postParameter PostParameter
	var processType string = "HTTP_SERVER"
	var responseTimeList []float64
	var responseTime string
	var lock sync.Mutex
	var count int64
	var config conf
	var logAbsPath string
	for _, logAbsPath = range properties {
		logAbsPath = strings.TrimSpace(strings.Replace(logAbsPath, "\n", "", -1))
		break
	}

	tailConfig := tail.Config{
		ReOpen:    true,
		Follow:    true,
		Location:  &tail.SeekInfo{Offset: 0, Whence: 2},
		MustExist: false,
		Poll:      true,
	}
	tails, err := tail.TailFile(logAbsPath, tailConfig)
	if err != nil {
		fmt.Println("tail file failed err:", err)
		return
	}

	//	workResultLock.Add(1)
	//    go func(ctx context.Context) {
	//        for {
	//            	preline ,_ := <- tails.Lines
	//				line <- preline.Text
	//            }
	//		workResultLock.Done()
	//    }(ctx)
	//	workResultLock.Wait()

START:
	fmt.Println("number")
	fmt.Println(runtime.NumGoroutine())
	fmt.Println("number")
	responseTimeList = []float64{}
	count = 0
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	stopCh := make(chan struct{})

	go func(ctx context.Context, stopCh <-chan struct{}) {
		for {
			select {
			case <-stopCh:
				fmt.Println("Recv stop signal")
				return

			default:
				preline, _ := <-tails.Lines
				count++
				lineText := fmt.Sprintf(preline.Text)
				lineDone := strings.TrimSpace(lineText)
				responseTime = strings.Split(lineDone, " ")[len(strings.Split(lineDone, " "))-1]
				if responseTime == "0.000" {
					continue
				} else {
					responsTimeFloat64, err := strconv.ParseFloat(responseTime, 64)
					if err != nil {
						continue
					}
					lock.Lock()
					responseTimeList = append(responseTimeList, responsTimeFloat64)
					lock.Unlock()
				}
			}
		}
	}(ctx, stopCh)

	select {
	case <-ctx.Done():
		close(stopCh)
		pare := &postParameter
		var machineId, logAbsPath string
		urlConfig := config.getConf()
		pare.LogAbsPath = logAbsPath
		pare.UrlConfig = urlConfig
		sort.Sort(FloatSlice(responseTimeList))
		if len(responseTimeList) == 0 {
			responseTimeList = append(responseTimeList, 0)
		}
		postResponseTimeList := responseTimeList
		postCount := count
		for machineId, _ = range properties {
			pare.MachineId = machineId
			pare.AnalysisAndPost(postResponseTimeList, postCount, processType)
		}
		fmt.Println("本次结束！")
		cancel()
	}
	goto START
}
