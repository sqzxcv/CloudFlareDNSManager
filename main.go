package main

import (
    "context"
    "flag"
    "fmt"
    cf "github.com/cloudflare/cloudflare-go"
    "github.com/spf13/viper"
    "github.com/sqzxcv/glog"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"
)

type Dns struct {
    Type      string      `mapstructure:"type"`
    Name      string      `mapstructure:"name"`
    Content   string      `mapstructure:"content"`
    Meta      interface{} `mapstructure:"meta"`
    Data      interface{} `mapstructure:"data"`
    ID        string      `mapstructure:"id"`
    ZoneID    string      `mapstructure:"zone_id"`
    ZoneName  string      `mapstructure:"zone_name"`
    Priority  uint16      `mapstructure:"priority"`
    TTL       int         `mapstructure:"ttl"`
    Proxied   bool        `mapstructure:"proxied"`
    Proxiable bool        `mapstructure:"proxiable"`
    Locked    bool        `mapstructure:"locked"`
    // 操作: create, update; update 如果记录不存在则创建,否则只更新.
    Operate string `mapstructure:operate`
}

type Config struct {
    Email  string `mapstructure:"email"`
    Key    string `mapstructure:"global_key"`
    AllDns []Dns  `mapstructure:"dns"`
}

func main() {
    //defer exception.CatchException("主线程出现异常")
    logsDir := GetExeDirectory() + "/logs"
    glog.SetConsole(true)
    glog.Info("日志目录:", logsDir)
    glog.SetRollingDaily(logsDir, "autodeploy.log", false)
    glog.Info(">>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>>> App 启动成功 <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<")
    var taskConfigPath = flag.String("c", "", "toml的task配置文件")
    flag.Parse()
    if len((*taskConfigPath)) == 0 {
        glog.Error("请指定task configViper")
        return
        //*taskConfigPath = getCurrentDirectory() + "/configViper.toml"
    }

    configViper := viper.New()
    configViper.SetConfigFile(*taskConfigPath)
    err := configViper.ReadInConfig()
    if err != nil {
        glog.Error("配置文件加载失败, 原因:", err.Error())

    }

    config := &Config{}
    if err := configViper.Unmarshal(config); err != nil {
        glog.Error("配置序列化失败, 原因:", err.Error())
        return
    }

    api, err := cf.New(config.Key, config.Email)
    // alternatively, you can use a scoped API token
    // api, err := cloudflare.NewWithAPIToken(os.Getenv("CLOUDFLARE_API_TOKEN"))
    if err != nil {
        glog.Error(err)
    }

    ctx := context.Background()
    for _, dns := range config.AllDns {
        root := rootDomain(dns.Name)
        zoneID, err := api.ZoneIDByName(root)

        if err != nil {
            glog.Error("获取zoneid失败:", err)
            continue
        }
        recs, err := api.DNSRecords(ctx, zoneID, cf.DNSRecord{
            Name: dns.Name,
        })
        if err != nil {
            glog.Error(err)
        }

        if strings.Compare(dns.Operate, "create") == 0 {

            _, err := api.CreateDNSRecord(ctx, zoneID, cf.DNSRecord{
                CreatedOn:  time.Now(),
                ModifiedOn: time.Now(),
                Type:       strings.TrimSpace(dns.Type),
                Name:       strings.TrimSpace(dns.Name),
                Content:    strings.TrimSpace(dns.Content),
                ID:         "",
                ZoneID:     zoneID,
                ZoneName:   dns.ZoneName,
                Priority:   &dns.Priority,
                TTL:        dns.TTL,
                Proxied:    &dns.Proxied,
            })
            if err != nil {
                glog.Error("保存dns失败:", err)
                continue
            }
        } else {
            if recs == nil || len(recs) == 0 {

                _, err := api.CreateDNSRecord(ctx, zoneID, cf.DNSRecord{
                    CreatedOn:  time.Now(),
                    ModifiedOn: time.Now(),
                    Type:       strings.TrimSpace(dns.Type),
                    Name:       strings.TrimSpace(dns.Name),
                    Content:    strings.TrimSpace(dns.Content),
                    ID:         "",
                    ZoneID:     zoneID,
                    ZoneName:   dns.ZoneName,
                    Priority:   &dns.Priority,
                    TTL:        dns.TTL,
                    Proxied:    &dns.Proxied,
                })
                if err != nil {
                    glog.Error("保存dns失败:", err)
                    continue
                }
            } else {
                for _, r := range recs {
                    //fmt.Printf("%s: %s\n", r.Name, r.Content)
                    r.Content = strings.TrimSpace(dns.Content)
                    r.Proxied = &dns.Proxied
                    r.Priority = &dns.Priority
                    r.TTL = dns.TTL

                    err = api.UpdateDNSRecord(ctx, zoneID, r.ID, r)
                    if err != nil {
                        glog.Error("保存dns失败:", err)
                        continue
                    }
                }
            }
        }
        glog.Info("dns[", dns.Name, "] 保存成功....")
    }
}

func rootDomain(domain string) string {

    arr := strings.Split(domain, ".")
    len := len(arr)
    root := fmt.Sprintf("%s.%s", arr[len-2], arr[len-1])
    return root
}

// 获取可执行文件所在目录
func GetExeDirectory() string {

    file, _ := exec.LookPath(os.Args[0])
    path, _ := filepath.Abs(file)
    index := strings.LastIndex(path, string(os.PathSeparator))
    ret := path[:index]

    return strings.Replace(ret, "\\", "/", -1) //将\替换成/
}
