package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/gin-gonic/gin"
	"gopkg.in/yaml.v2"
)

type Endpoint struct {
	Path        string `yaml:"path"`
	Method      string `yaml:"method"`
	ResponseFile string `yaml:"responseFile"`
}

type Service struct {
	Name      string     `yaml:"name"`
	BasePath  string     `yaml:"basePath"`
	Endpoints []Endpoint `yaml:"endpoints"`
}

type Config struct {
	Services []Service `yaml:"services"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	
	var config Config
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	
	return &config, nil
}

func readJSONFile(filePath string) ([]byte, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}
	
	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, err
	}
	
	return data, nil
}

func setupRouter(config *Config) *gin.Engine {
	r := gin.Default()
	
	for _, service := range config.Services {
		for _, endpoint := range service.Endpoints {
			fullPath := service.BasePath + endpoint.Path
			responseFile := endpoint.ResponseFile
			
			switch endpoint.Method {
			case "GET":
				r.GET(fullPath, func(c *gin.Context) {
					data, err := readJSONFile(responseFile)
					if err != nil {
						c.JSON(500, gin.H{"error": err.Error()})
						return
					}
					c.Data(200, "application/json", data)
				})
			case "POST":
				r.POST(fullPath, func(c *gin.Context) {
					data, err := readJSONFile(responseFile)
					if err != nil {
						c.JSON(500, gin.H{"error": err.Error()})
						return
					}
					c.Data(200, "application/json", data)
				})
			}
		}
	}
	
	return r
}

func main() {
	config, err := loadConfig("./config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	var configLock sync.RWMutex
	r := setupRouter(config)
	
	// 创建文件监控
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()
	
	// 添加配置文件到监控
	err = watcher.Add("./config.yaml")
	if err != nil {
		log.Printf("Failed to watch config file: %v", err)
	}
	
	// 添加所有JSON响应文件到监控
	for _, service := range config.Services {
		for _, endpoint := range service.Endpoints {
			err = watcher.Add(endpoint.ResponseFile)
			if err != nil {
				log.Printf("Failed to watch response file %s: %v", endpoint.ResponseFile, err)
			}
		}
	}
	
	// 启动文件监控协程
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Printf("File modified: %s", event.Name)
					
					// 重新加载配置
					configLock.Lock()
					newConfig, err := loadConfig("./config.yaml")
					if err != nil {
						log.Printf("Failed to reload config: %v", err)
						configLock.Unlock()
						continue
					}
					
					// 更新路由
					*config = *newConfig
					configLock.Unlock()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()
	
	fmt.Println("Starting mock server on :8080")
	r.Run(":8080")
}