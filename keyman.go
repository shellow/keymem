package keyman

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/bluele/gcache"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type Keyman struct {
	Keypre     string
	RedisPool  *redis.Pool
	TokenCache gcache.Cache
	TokenTime  time.Duration
}

type HKey struct {
	Key  string `form:"key" json:"key" xml:"key"`
	Name string `form:"name" json:"name" xml:"name"`
}

type Key struct {
	Key    string `form:"key" json:"key" xml:"key" binding:"required"`
	Expday int    `form:"expday" json:"expday" xml:"expday"`
	Number int64  `form:"number" json:"number" xml:"number"`
}

type TokenInfo struct {
	Key   string `form:"key" json:"key" xml:"key" binding:"required"`
	Route string `form:"Route" json:"Route" xml:"Route"`
}

func (tokenInfo *TokenInfo) Marshal() ([]byte, error) {
	return json.Marshal(*tokenInfo)
}

func (tokenInfo *TokenInfo) Unmarshal(data []byte) error {
	return json.Unmarshal(data, tokenInfo)
}

func (keyman *Keyman) keyAddPre(key string) string {
	return keyman.Keypre + key
}

func genCountKey(path, key string) string {
	return path + "-" + key
}

func genTotleCountKey(path, key string) string {
	return path + "-totle-" + key
}

func (keyman *Keyman) keyDelPre(key string) string {
	return strings.Replace(key, keyman.Keypre, "", 1)
}

func (keyman *Keyman) StrToPriv(key string) *ecdsa.PrivateKey {
	key = keyman.keyDelPre(key)
	return StrToPriv(key)
}

func StrToPriv(key string) *ecdsa.PrivateKey {
	priv := new(ecdsa.PrivateKey)
	d := big.NewInt(0)
	d.SetString(key, 0)
	priv.D = d
	priv.PublicKey.Curve = crypto.S256()
	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(priv.D.Bytes())
	return priv
}

func (keyman *Keyman) InitHandle(router *gin.Engine) {
	router.POST("/keymem/enable", keyman.Enable)
	router.POST("/keymem/addkey", keyman.Addkey)
	router.POST("/keymem/delkey", keyman.Delkey)
	router.POST("/keymem/getkey", keyman.Getkey)
	router.GET("/keymem/listkey", keyman.Listkey)
	router.POST("/keymem/diskey", keyman.Diskey)
	router.GET("/keymem/keyaddr", keyman.GetKeyAddr)
	router.GET("/keymem/getownkey", keyman.Getownkey)

	router.POST("/keymem/addcount", keyman.AddCount)
	router.POST("/keymem/getcount", keyman.GetCount)
	router.GET("/keymem/getkeyexpdate", keyman.GetKeyExpdate)
	router.POST("/keymem/addtotlecount", keyman.AddTotleCount)
}

func (keyman *Keyman) GetPriv(c *gin.Context) (*ecdsa.PrivateKey, error) {
	key := c.GetHeader("key")
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	isExist, err := redis.Int(redisConn.Do("HEXISTS", "keys", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if isExist == 0 {
		return nil, nil
	}

	priv := keyman.StrToPriv(key)
	return priv, nil
}

func (keyman *Keyman) GetManPriv(c *gin.Context) (*ecdsa.PrivateKey, error) {
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	key := c.GetHeader("key")
	isExist, err := redis.Int(redisConn.Do("HEXISTS", "mkeys", key))
	if err == redis.ErrNil {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	if isExist == 0 {
		return nil, nil
	}
	priv := keyman.StrToPriv(key)
	return priv, nil
}

func (keyman *Keyman) Enable(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	var key Key

	err = c.BindJSON(&key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	isExist, err := redis.Int(redisConn.Do("HEXISTS", "keys", keyman.keyAddPre(key.Key)))
	if err == redis.ErrNil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if isExist == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	}

	_, err = redisConn.Do("SET", keyman.keyAddPre(key.Key), key.Number)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	exptime := time.Now()
	exptime = exptime.Add(time.Duration(key.Expday) * time.Hour * 24)
	sec := exptime.Unix()
	_, err = redisConn.Do("EXPIREAT", keyman.keyAddPre(key.Key), sec)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"expdate": exptime.Format("2006-01-02T15:04:05"),
		"number":  key.Number,
	})
}

func (keyman *Keyman) Addkey(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	var key HKey
	err = c.BindJSON(&key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	if len(key.Key) < 70 {
		k, _ := crypto.GenerateKey()
		key.Key = k.D.String()
	}

	_, err = redisConn.Do("HSET", "keys", keyman.keyAddPre(key.Key), key.Name)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"key":    key.Key,
		"name":   key.Name,
	})

}

func (keyman *Keyman) Delkey(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	var key HKey
	err = c.BindJSON(&key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	if len(key.Key) < 70 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "too less",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	_, err = redisConn.Do("HDEL", "keys", keyman.keyAddPre(key.Key))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"key":    key.Key,
	})
}

func (keyman *Keyman) Getkey(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	var key HKey
	err = c.BindJSON(&key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	name, err := redis.String(redisConn.Do("HGET", "keys", keyman.keyAddPre(key.Key)))
	if err == redis.ErrNil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	sec, err := redis.Int(redisConn.Do("TTL", keyman.keyAddPre(key.Key)))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if sec < 0 {
		sec = 0
	}

	number, err := redis.Int(redisConn.Do("GET", keyman.keyAddPre(key.Key)))
	if err == redis.ErrNil {
		number = 0
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	expdate := time.Now().Add(time.Duration(sec) * time.Second)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"name":    name,
		"sec":     sec,
		"expdate": expdate,
		"number":  number,
	})

}

func (keyman *Keyman) Listkey(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	keys, err := redis.Strings(redisConn.Do("HKEYS", "keys"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	var retkeys []string
	for i := 0; i < len(keys); i++ {
		if strings.HasPrefix(keys[i], keyman.Keypre) {
			retkeys = append(retkeys, keyman.keyDelPre(keys[i]))
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"keys":   retkeys,
	})

}

func (keyman *Keyman) Diskey(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	var key HKey
	err = c.BindJSON(&key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	if len(key.Key) < 70 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "too less",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	_, err = redisConn.Do("DEL", keyman.keyAddPre(key.Key))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"key":    key.Key,
	})
}

func (keyman *Keyman) Getownkey(c *gin.Context) {
	if !keyman.IsKeyValid(c) {
		return
	}

	key := c.GetHeader("key")

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	name, err := redis.String(redisConn.Do("HGET", "keys", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	sec, err := redis.Int(redisConn.Do("TTL", keyman.keyAddPre(key)))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if sec < 0 {
		sec = 0
	}

	number, err := redis.Int(redisConn.Do("GET", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		number = 0
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	expdate := time.Now().Add(time.Duration(sec) * time.Second)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"name":    name,
		"sec":     sec,
		"expdate": expdate,
		"number":  number,
	})

}

func (keyman *Keyman) AddCount(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	key := c.Request.FormValue("key")
	if strings.EqualFold("", key) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require key",
		})
		return
	}

	reqpath := c.Request.FormValue("reqpath")
	if strings.EqualFold("", reqpath) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require reqpath",
		})
		return
	}

	count := c.Request.FormValue("count")
	if strings.EqualFold("", count) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require count",
		})
		return
	}

	countInt, err := strconv.Atoi(count)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "count error",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	isExist, err := redis.Int(redisConn.Do("HEXISTS", "keys", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if isExist == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	}

	_, err = redisConn.Do("INCRBY", genCountKey(reqpath, key), countInt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	_, err = redisConn.Do("INCRBY", genTotleCountKey(reqpath, key), countInt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (keyman *Keyman) AddTotleCount(c *gin.Context) {
	priv, err := keyman.GetManPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}

	key := c.Request.FormValue("key")
	if strings.EqualFold("", key) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require key",
		})
		return
	}

	reqpath := c.Request.FormValue("reqpath")
	if strings.EqualFold("", reqpath) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require reqpath",
		})
		return
	}

	count := c.Request.FormValue("count")
	if strings.EqualFold("", count) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require count",
		})
		return
	}

	countInt, err := strconv.Atoi(count)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "count error",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	isExist, err := redis.Int(redisConn.Do("HEXISTS", "keys", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if isExist == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key not exist",
		})
		return
	}

	_, err = redisConn.Do("INCRBY", genTotleCountKey(reqpath, key), countInt)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (keyman *Keyman) GetCount(c *gin.Context) {
	if !keyman.IsKeyValid(c) {
		return
	}

	key := c.GetHeader("key")

	reqpath := c.Request.FormValue("reqpath")
	if strings.EqualFold("", reqpath) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "require reqpath",
		})
		return
	}

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	number, err := redis.Int(redisConn.Do("GET", genCountKey(reqpath, key)))
	if err == redis.ErrNil {
		number = 0
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	totleNumber, err := redis.Int(redisConn.Do("GET", genTotleCountKey(reqpath, key)))
	if err == redis.ErrNil {
		number = 0
	} else if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"number": number,
		"totle":  totleNumber,
	})
}

func (keyman *Keyman) GetKeyExpdate(c *gin.Context) {
	if !keyman.IsKeyValid(c) {
		return
	}

	key := c.GetHeader("key")

	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	sec, err := redis.Int(redisConn.Do("TTL", keyman.keyAddPre(key)))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if sec < 0 {
		sec = 0
	}

	expdate := time.Now().Add(time.Duration(sec) * time.Second)

	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"sec":     sec,
		"expdate": expdate,
	})
}

func (keyman *Keyman) CheckKey(key string) error {
	// is key valid
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	num, err := redis.Int(redisConn.Do("GET", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		return errors.New("Expiry date")
	} else if err != nil {
		return err
	}
	if num <= 0 {
		return errors.New("Exceed quota of use")
	}
	return nil
}

func (keyman *Keyman) CheckPathKeyCount(reqpath, key string) error {
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	number, err := redis.Int(redisConn.Do("GET", genCountKey(reqpath, key)))
	if err != nil {
		return errors.New("Exceed quota of use")
	}
	if number <= 0 {
		return errors.New("Exceed quota of use")
	}
	return err
}

func (keyman *Keyman) DecPathKeyCount(reqpath, key string) error {
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()

	_, err := redisConn.Do("DECR", genCountKey(reqpath, key))
	if err != nil {
		return err
	}
	return nil
}

func (keyman *Keyman) DecPathKeyCountHandle(c *gin.Context) bool {
	key := c.GetHeader("key")
	err := keyman.DecPathKeyCount(c.Request.URL.Path, key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "dec count failed",
		})
		return false
	}
	return true
}

func (keyman *Keyman) CheckKeyOnlytime(key string) error {
	// is key valid
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	_, err := redis.Int(redisConn.Do("GET", keyman.keyAddPre(key)))
	if err == redis.ErrNil {
		return errors.New("Expiry date")
	} else if err != nil {
		return err
	}
	return nil
}

func (keyman *Keyman) IsKeyValid(c *gin.Context) bool {
	priv, err := keyman.GetPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return false
	}

	// is key valid
	key := priv.D.String()
	err = keyman.CheckKey(key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	return true
}

func (keyman *Keyman) IsKeyValidOnlytime(c *gin.Context) bool {
	priv, err := keyman.GetPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return false
	}

	// is key valid
	key := priv.D.String()
	err = keyman.CheckKeyOnlytime(key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	return true
}

func (keyman *Keyman) IsKeyValidRet(c *gin.Context) (*ecdsa.PrivateKey, bool) {
	priv, err := keyman.GetPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return priv, false
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return priv, false
	}

	// is key valid
	key := priv.D.String()
	err = keyman.CheckKey(key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return priv, false
	}
	return priv, true
}

func (keyman *Keyman) IsPathKeyValid(c *gin.Context) bool {
	priv, err := keyman.GetPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return false
	}

	// is key valid
	key := priv.D.String()
	err = keyman.CheckPathKeyCount(c.Request.URL.Path, key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return false
	}
	return true
}

func (keyman *Keyman) GetKeyAddr(c *gin.Context) {
	priv, err := keyman.GetPriv(c)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "access denied",
		})
		return
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	addrStr := AddrToStr(&addr)
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"address": addrStr,
	})
	return
}

func (keyman *Keyman) DecKeyNum(key string) error {
	redisConn := keyman.RedisPool.Get()
	defer redisConn.Close()
	_, err := redisConn.Do("DECR", keyman.keyAddPre(key))
	return err
}

// token route access
func (keyman *Keyman) GetToken(c *gin.Context) {
	if !keyman.IsKeyValid(c) {
		return
	}

	key := c.GetHeader("key")

	priv := keyman.StrToPriv(key)
	if priv == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "key error",
		})
		return
	}

	err := keyman.CheckKey(key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
	}

	token := MakeToken(priv)
	tokeninfo := new(TokenInfo)
	tokeninfo.Key = key
	tokeninfo.Route = c.Request.URL.Path
	b, err := tokeninfo.Marshal()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return
	}
	keyman.TokenCache.SetWithExpire(token, b, keyman.TokenTime)
	c.Header("token", token)
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
	})
}

func (keyman *Keyman) CheckToken(c *gin.Context) *TokenInfo {
	token := c.GetHeader("token")
	b, err := keyman.TokenCache.Get(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "token not exist",
		})
		return nil
	}

	tokeninfo := new(TokenInfo)
	tokeninfo.Unmarshal(b.([]byte))

	if !strings.HasPrefix(c.Request.URL.Path, tokeninfo.Route) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "route denied",
		})
		return nil
	}

	err = keyman.CheckKey(tokeninfo.Key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return nil
	}

	return tokeninfo
}

func (keyman *Keyman) CheckGetToken(c *gin.Context) *TokenInfo {
	token, _ := c.GetQuery("token")
	b, err := keyman.TokenCache.Get(token)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "token not exist",
		})
		return nil
	}

	tokeninfo := new(TokenInfo)
	tokeninfo.Unmarshal(b.([]byte))

	if !strings.HasPrefix(c.Request.URL.Path, tokeninfo.Route) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": "route denied",
		})
		return nil
	}

	err = keyman.CheckKey(tokeninfo.Key)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "error",
			"message": err.Error(),
		})
		return nil
	}

	return tokeninfo
}

func MakeToken(priv *ecdsa.PrivateKey) string {
	b := make([]byte, 32)
	rand.Read(b)
	sig, err := crypto.Sign(b, priv)
	if err != nil {
		return ""
	}
	//bs := fmt.Sprintf("%x", b)
	//sigs := fmt.Sprintf("%x", sig)
	//ret := bs+sigs
	return fmt.Sprintf("%x%x", b, sig)
}

func TokenToPubStr(token string) (string, error) {
	if len(token) != 194 {
		return "", errors.New("tooken error")
	}
	hash, err := hex.DecodeString(token[0:64])
	if err != nil {
		return "", err
	}
	sig, err := hex.DecodeString(token[64:194])
	if err != nil {
		return "", err
	}
	pub, err := crypto.Ecrecover(hash, sig)
	return fmt.Sprintf("%x", pub), err
}

func TokenToPub(token string) (*ecdsa.PublicKey, error) {
	if len(token) != 194 {
		return nil, errors.New("tooken error")
	}
	hash, err := hex.DecodeString(token[0:64])
	if err != nil {
		return nil, err
	}
	sig, err := hex.DecodeString(token[64:194])
	if err != nil {
		return nil, err
	}
	pub, err := crypto.SigToPub(hash, sig)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

func TokenToAddr(token string) (*common.Address, error) {
	pub, err := TokenToPub(token)
	if err != nil {
		return nil, err
	}
	addr := crypto.PubkeyToAddress(*pub)
	return &addr, nil
}

func AddrToStr(addr *common.Address) string {
	return strings.ToLower(strings.Replace(addr.String(), "0x", "", 1))
}

func TokenToAddrStr(token string) (string, error) {
	addr, err := TokenToAddr(token)
	if err != nil {
		return "", err
	}
	addrStr := AddrToStr(addr)
	return addrStr, nil
}

func KeyToAddr(key string) *common.Address {
	priv := StrToPriv(key)
	if priv == nil {
		return nil
	}
	addr := crypto.PubkeyToAddress(priv.PublicKey)
	return &addr
}

func KeyToAddrStr(key string) string {
	addr := KeyToAddr(key)
	if addr == nil {
		return ""
	}
	addrStr := AddrToStr(addr)
	return addrStr
}
