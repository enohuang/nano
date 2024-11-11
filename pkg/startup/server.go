package startup

import (
	"fmt"
	"os"
	"runtime/pprof"
	"strings"
	"time"

	nano "gnano"
	"gnano/component"
	"gnano/pkg/option"

	"github.com/spf13/viper"
	"github.com/urfave/cli"
)

// Startup启动
// Example:
// func main() {
// 	app := cli.NewApp()
// 	// base application info
// 	app.Name = " server"
// 	app.Author = ""
// 	app.Version = "0.0.1"
// 	app.Copyright = "game team reserved"
// 	app.Usage = " server"
// 	// flags
// 	app.Flags = []cli.Flag{
// 		cli.StringFlag{
// 			Name:  "c",
// 			Value: "./configs",
// 			Usage: "load configuration from `FILE`",
// 		},
// 		cli.BoolFlag{
// 			Name:  "cpuprofile",
// 			Usage: "enable cpu profile",
// 		},
// 	}
// 	app.Action = serve
// 	err := app.Run(os.Args)
// 	if err != nil {
// 		fmt.Printf("%s run err %s", app.Name, err.Error())
// 	}
// }

//	func serve(c *cli.Context) error {
//		//设置读取的文件类型
//		viper.SetConfigType("toml")
//		viper.AddConfigPath(c.String("c"))
//		viper.SetConfigName("config.toml")
//		err := viper.ReadInConfig()
//		if err != nil {
//			log.Fatalf("ReadInConfig", err.Error())
//		}
//		//性能检测
//		if c.Bool("debug.cpuprofile") {
//			filename := fmt.Sprintf("cpuprofile-%d.pprof", time.Now().Unix())
//			f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, os.ModePerm)
//			if err != nil {
//				panic(err)
//			}
//			pprof.StartCPUProfile(f)
//			defer pprof.StopCPUProfile()
//		}
//		components := comps.Comps(NewComponent())
//		//startup.Startup(components)
//		wg := sync.WaitGroup{}
//		wg.Add(1)
//		go func() { defer wg.Done(); startup.Startup(components) }()
//		wg.Wait()
//		return nil
//	}
//
// AppEnv  为 urfave/cli 方式启动的APP 设置 环境变量
// urfave/cli is a declarative, simple, fast, and fun package for building command line tools in Go featuring
func WithAppEnv(c *cli.Context, configType, configName string) {

	flags := []cli.Flag{
		cli.StringFlag{
			Name:  "c",
			Value: "../configs",
			Usage: "load configuration from `FILE`",
		},
		cli.BoolFlag{
			Name:  "cpuprofile",
			Usage: "enable cpu profile",
		},
		cli.StringFlag{
			Name:  "host",
			Value: "",
			Usage: "host",
		},
		cli.IntFlag{
			Name:  "port",
			Value: 0,
			Usage: "port",
		},
		cli.IntFlag{
			Name:  "snowflake_node",
			Value: 0,
			Usage: "snowflake_node",
		},
		cli.StringFlag{
			Name:  "client_addr",
			Value: "",
			Usage: "client_addr",
		},
	}

	if c.App.Flags == nil {
		c.App.Flags = flags
	} else {
		c.App.Flags = append(c.App.Flags, flags...)
	}

	//调整配置文件读取路径
	viper.SetConfigType(configType)
	viper.AddConfigPath(c.String("c"))
	viper.SetConfigName(configName)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
	c.App.Name = viper.GetString("app.name")
	c.App.Author = viper.GetString("app.author")
	c.App.Version = viper.GetString("app.version")
	c.App.Copyright = viper.GetString("app.copyright")
	c.App.Usage = viper.GetString("app.usage")
	if c.Bool("cpuprofile") {
		filename := fmt.Sprintf("cpuprofile-%d.pprof", time.Now().Unix())
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, os.ModePerm)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
}

func Startup(components *component.Components, c *cli.Context, ops ...nano.Option) {
	//启动节点
	var options = make([]nano.Option, 0)
	if !viper.GetBool("cluster.single") {
		options = option.ClusterOptions(components, c)
	} else {
		//单节点启动
		options = option.NodeOptions(components, c)
	}

	for _, v := range ops {
		options = append(options, v)
	}
	var host = c.String("host")
	var port = c.Int("port")
	addr := fmt.Sprintf("%s:%d", host, port)
	if len(strings.TrimSpace(host)) == 0 || port == 0 {
		addr = fmt.Sprintf("%s:%d", viper.GetString("network.host"), viper.GetInt("network.port"))
	}
	fmt.Printf("\t\t\t\t\t\t\t\t\t %s SERVICE STARTUP\n", strings.ToUpper(c.App.Name))
	nano.Listen(addr,
		options...,
	)
}
