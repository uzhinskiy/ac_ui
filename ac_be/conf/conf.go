package conf

import (
	"io/ioutil"
	"log"
	"strings"
)

type ConfigType map[string]string

var Config ConfigType

func (cfg ConfigType) Parse(configfile string) (err error) {
	rawBytes, err := ioutil.ReadFile(configfile)
	if err != nil {
		log.Fatal(err)
	}

	text := string(rawBytes)
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		if !strings.HasPrefix(line, "#") && !strings.HasPrefix(line, ";") {
			fields := strings.Split(line, "=")
			if len(fields) == 2 {
				tmp := strings.TrimSpace(fields[1])
				j1 := strings.Index(tmp, "\"")
				j2 := strings.LastIndex(tmp, "\"")
				if j1 == 0 && j2 > 0 {
					cfg[strings.TrimSpace(fields[0])] = tmp[j1+1 : j2]
				} else if j1 == -1 && j2 == -1 {
					cfg[strings.TrimSpace(fields[0])] = tmp
				} else {
					log.Println("invalid option")
				}
			}
		}
	}
	return
}
