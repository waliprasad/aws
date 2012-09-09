package glacier

import (
	"../../aws"
	"fmt"
	"testing"
)

func TestList(t *testing.T) {
	secret, access := aws.KeysFromEnviroment()
	connection := NewGlacierConnection(secret, access, aws.USEast)
	_, result, err := connection.ListVaults(1000, "")
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println(result)
}
