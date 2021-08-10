package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/shellow/keyman"
	"github.com/urfave/cli"
	"go.uber.org/zap"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strconv"
	"time"
)

var Logger *zap.SugaredLogger

var HOSTURL string
var KEY string

func main() {
	app := cli.NewApp()
	app.Name = "Key manage"
	app.Usage = "Key manage"
	app.Version = "1.0.0"
	app.Flags = []cli.Flag{
		//cli.IntFlag{
		//	Name:  "port, p",
		//	Value: 8000,
		//	Usage: "listening port",
		//},
		cli.StringFlag{
			Name:        "surl, s",
			Value:       "http://127.0.0.1",
			Usage:       "server url",
			Destination: &HOSTURL,
		},
		cli.StringFlag{
			Name:        "key, k",
			Value:       "key",
			Usage:       "server key",
			Destination: &KEY,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:     "list",
			Usage:    "list keys",
			Category: "manage",
			Action:   listkey,
		},
		{
			Name:     "addmankey",
			Usage:    "add manage keys",
			Category: "manage",
			Action:   addmemkey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "raddr",
					Value: "1",
					Usage: "redis address",
				},
				cli.StringFlag{
					Name:  "rpass",
					Value: "1",
					Usage: "redis password",
				},
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for add",
				},
				cli.StringFlag{
					Name:  "hkeyname, kn",
					Value: "key",
					Usage: "key name",
				},
			},
		},
		{
			Name:     "add",
			Usage:    "add key",
			Category: "manage",
			Action:   addkey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for add",
				},
				cli.StringFlag{
					Name:  "hkeyname, kn",
					Value: "key",
					Usage: "key name",
				},
			},
		},
		{
			Name:     "enable",
			Usage:    "enable key",
			Category: "manage",
			Action:   enablekey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for add",
				},
				cli.StringFlag{
					Name:  "day",
					Value: "10",
					Usage: "Time limit",
				},
				cli.StringFlag{
					Name:  "num",
					Value: "10",
					Usage: "Limit of times",
				},
			},
		},
		{
			Name:     "get",
			Usage:    "get key",
			Category: "manage",
			Action:   getkey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for get",
				},
			},
		},
		{
			Name:     "dis",
			Usage:    "dis key",
			Category: "manage",
			Action:   diskey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for dis",
				},
			},
		},
		{
			Name:     "del",
			Usage:    "del key",
			Category: "manage",
			Action:   delkey,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for del",
				},
			},
		},
		{
			Name:     "token",
			Usage:    "get token",
			Category: "manage",
			Action:   gettoken,
		},
		{
			Name:     "address",
			Usage:    "get key address",
			Category: "manage",
			Action:   keyaddr,
		},
		{
			Name:     "getownkey",
			Usage:    "get own key",
			Category: "manage",
			Action:   getownkey,
		},
		{
			Name:     "addcount",
			Usage:    "add key path count",
			Category: "manage",
			Action:   addcount,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for add",
				},
				cli.StringFlag{
					Name:  "reqpath",
					Value: "/test",
					Usage: "uri path",
				},
				cli.IntFlag{
					Name:  "count",
					Value: 10,
					Usage: "quota for use",
				},
			},
		},
		{
			Name:     "addtotlecount",
			Usage:    "add key path totle count",
			Category: "manage",
			Action:   addtotlecount,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "hkey, hk",
					Value: "1",
					Usage: "key for add",
				},
				cli.StringFlag{
					Name:  "reqpath",
					Value: "/test",
					Usage: "uri path",
				},
				cli.IntFlag{
					Name:  "count",
					Value: 10,
					Usage: "quota for use",
				},
			},
		},
		{
			Name:     "getcount",
			Usage:    "get key path count",
			Category: "manage",
			Action:   getcount,
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "reqpath",
					Value: "/test",
					Usage: "uri path",
				},
			},
		},
		{
			Name:     "getkeyexpdate",
			Usage:    "get key expdate",
			Category: "manage",
			Action:   getkeyexpdate,
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func listkey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/listkey"
	req, err := http.NewRequest("GET", murl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func addmemkey(c *cli.Context) error {
	raddr := c.String("raddr")
	rpass := c.String("rpass")
	hkey := c.String("hkey")
	hkeyname := c.String("hkeyname")
	con, err := redis.Dial("tcp", raddr,
		redis.DialPassword(rpass),
		redis.DialDatabase(0),
		redis.DialConnectTimeout(3*time.Second),
		redis.DialReadTimeout(3*time.Second),
		redis.DialWriteTimeout(3*time.Second))
	if err != nil {
		return err
	}

	_, err = con.Do("HSET", "mkeys", hkey, hkeyname)
	if err != nil {
		return err
	}

	fmt.Println("ok")
	return nil
}

func addkey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/addkey"
	var b bytes.Buffer
	var key keyman.HKey
	key.Key = c.String("hkey")
	key.Name = c.String("hkeyname")
	bj, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(bj)
	req, err := http.NewRequest("POST", murl, &b)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func enablekey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/enable"
	var b bytes.Buffer
	var key keyman.Key
	key.Key = c.String("hkey")
	key.Expday = c.Int("day")
	key.Number = c.Int64("num")
	bj, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(bj)
	req, err := http.NewRequest("POST", murl, &b)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func getkey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/getkey"
	var b bytes.Buffer
	var key keyman.HKey
	key.Key = c.String("hkey")
	bj, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(bj)
	req, err := http.NewRequest("POST", murl, &b)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func diskey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/diskey"
	var b bytes.Buffer
	var key keyman.HKey
	key.Key = c.String("hkey")
	bj, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(bj)
	req, err := http.NewRequest("POST", murl, &b)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func delkey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/delkey"
	var b bytes.Buffer
	var key keyman.HKey
	key.Key = c.String("hkey")
	bj, err := json.Marshal(key)
	if err != nil {
		return err
	}
	b.Write(bj)
	req, err := http.NewRequest("POST", murl, &b)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func gettoken(c *cli.Context) error {
	murl := c.GlobalString("surl")
	req, err := http.NewRequest("PUT", murl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	fmt.Println(res.Header.Get("token"))
	return nil
}

func keyaddr(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/keyaddr"
	req, err := http.NewRequest("GET", murl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func getownkey(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/getownkey"
	req, err := http.NewRequest("GET", murl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func addcount(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/addcount"

	key := c.String("hkey")
	reqpath := c.String("reqpath")
	count := c.Int("count")

	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("key", key)
	_ = writer.WriteField("reqpath", reqpath)
	_ = writer.WriteField("count", strconv.Itoa(count))
	err := writer.Close()
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, murl, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("key", KEY)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func addtotlecount(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/addtotlecount"

	key := c.String("hkey")
	reqpath := c.String("reqpath")
	count := c.Int("count")

	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("key", key)
	_ = writer.WriteField("reqpath", reqpath)
	_ = writer.WriteField("count", strconv.Itoa(count))
	err := writer.Close()
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, murl, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("key", KEY)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func getcount(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/getcount"

	reqpath := c.String("reqpath")

	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("reqpath", reqpath)
	err := writer.Close()
	if err != nil {
		return err
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, murl, payload)

	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("key", KEY)

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}

func getkeyexpdate(c *cli.Context) error {
	murl := c.GlobalString("surl")
	murl = murl + "/keymem/getkeyexpdate"
	req, err := http.NewRequest("GET", murl, nil)
	if err != nil {
		return err
	}
	req.Header.Set("key", KEY)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return err
	}
	fmt.Println(string(body))
	return nil
}
