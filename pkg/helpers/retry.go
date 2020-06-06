package helpers

import (
	"fmt"
	"time"

	"github.com/jenkins-x/jx/v2/pkg/log"
)

// Retry executes a given function and reties 'attempts' times with a delay of 'sleep' between the executions
func Retry(attempts int, sleep time.Duration, call func() error) (err error) {
	for i := 0; ; i++ {
		err = call()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)

		log.Logger().Warnf("\nretrying after error:%s\n", err)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}
