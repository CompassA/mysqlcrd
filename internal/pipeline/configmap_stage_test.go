/*
 * @Author: Tomato
 * @Date: 2026-04-23 01:14:47
 */
package pipeline

import (
	"testing"

	"github.com/mysqlcrd/pkg/utils"
)

func TestNewConfinMapStage(t *testing.T) {
	stage, err := NewConfinMapStage("../../config/mysqlconf/")
	if err != nil {
		t.Fatalf("NewConfigMapState() failed: %v", err)
	}

	for _, name := range utils.FileNameArr {
		if _, ok := stage.Files[name]; !ok {
			t.Fatalf("file %s not found", name)
		}
	}
}
