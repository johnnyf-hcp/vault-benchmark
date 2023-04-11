package benchmarktests

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/hashicorp/vault/api"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const (
	KVV2ReadTestType    = "kvv2_read"
	KVV2WriteTestType   = "kvv2_write"
	KVV2ReadTestMethod  = "GET"
	KVV2WriteTestMethod = "POST"
)

func init() {
	TestList[KVV2ReadTestType] = func() BenchmarkBuilder {
		return &KVV2Test{action: "read"}
	}
	TestList[KVV2WriteTestType] = func() BenchmarkBuilder {
		return &KVV2Test{action: "write"}
	}
}

type KVV2Test struct {
	pathPrefix string
	header     http.Header
	config     *KVV2TestConfig
	action     string
	numKVs     int
	kvSize     int
}

type KVV2TestConfig struct {
	Config *KVV2Config `hcl:"config,block"`
}

type KVV2Config struct {
	KVSize int `hcl:"kvsize,optional"`
	NumKVs int `hcl:"numkvs,optional"`
}

func (k *KVV2Test) ParseConfig(body hcl.Body) error {
	k.config = &KVV2TestConfig{
		Config: &KVV2Config{
			KVSize: 1,
			NumKVs: 1000,
		},
	}

	diags := gohcl.DecodeBody(body, nil, k.config)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	return nil
}

func (k *KVV2Test) read(client *api.Client) vegeta.Target {
	secnum := int(1 + rand.Int31n(int32(k.numKVs)))
	return vegeta.Target{
		Method: "GET",
		URL:    client.Address() + k.pathPrefix + "/data/secret-" + strconv.Itoa(secnum),
		Header: k.header,
	}
}

func (k *KVV2Test) write(client *api.Client) vegeta.Target {
	secnum := int(1 + rand.Int31n(int32(k.numKVs)))
	value := strings.Repeat("a", k.kvSize)
	return vegeta.Target{
		Method: "POST",
		URL:    client.Address() + k.pathPrefix + "/data/secret-" + strconv.Itoa(secnum),
		Header: k.header,
		Body:   []byte(`{"data": {"foo": "` + value + `"}}`),
	}
}

func (k *KVV2Test) Target(client *api.Client) vegeta.Target {
	switch k.action {
	case "write":
		return k.write(client)
	default:
		return k.read(client)
	}
}

func (k *KVV2Test) GetTargetInfo() TargetInfo {
	var method string
	switch k.action {
	case "write":
		method = KVV2WriteTestMethod
	default:
		method = KVV2ReadTestMethod
	}
	tInfo := TargetInfo{
		method:     method,
		pathPrefix: k.pathPrefix,
	}
	return tInfo
}

func (k *KVV2Test) Cleanup(client *api.Client) error {
	_, err := client.Logical().Delete(strings.Replace(k.pathPrefix, "/v1/", "/sys/mounts/", 1))

	if err != nil {
		return fmt.Errorf("error cleaning up mount: %v", err)
	}
	return nil
}

func (k *KVV2Test) Setup(client *api.Client, randomMountName bool, mountName string) (BenchmarkBuilder, error) {
	var err error
	mountPath := mountName
	config := k.config.Config

	if randomMountName {
		mountPath, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	err = client.Sys().Mount(mountPath, &api.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error mounting kvv2: %v", err)
	}

	secval := map[string]interface{}{
		"data": map[string]interface{}{
			"foo": 1,
		},
	}

	// TODO: Find more deterministic way of avoiding this
	// Avoid error of the form:
	// * Upgrading from non-versioned to versioned data. This backend will be unavailable for a brief period and will resume service shortly.
	time.Sleep(2 * time.Second)

	for i := 1; i <= config.NumKVs; i++ {
		_, err = client.Logical().Write(mountPath+"/data/secret-"+strconv.Itoa(i), secval)
		if err != nil {
			return nil, fmt.Errorf("error writing kv: %v", err)
		}
	}

	return &KVV2Test{
		pathPrefix: "/v1/" + mountPath,
		header:     http.Header{"X-Vault-Token": []string{client.Token()}, "X-Vault-Namespace": []string{client.Headers().Get("X-Vault-Namespace")}},
		numKVs:     k.config.Config.NumKVs,
		kvSize:     k.config.Config.KVSize,
	}, nil
}

func (k *KVV2Test) Flags(fs *flag.FlagSet) {}