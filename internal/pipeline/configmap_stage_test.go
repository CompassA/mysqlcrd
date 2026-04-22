/*
 * @Author: Tomato
 * @Date: 2026-04-23 01:14:47
 */
package pipeline

import (
	"testing"
)

func TestNewConfinMapStage(t *testing.T) {
	stage, err := NewConfinMapStage("../../config/mysqlconf/")
	if err != nil {
		t.Fatalf("NewConfigMapState() failed: %v", err)
	}

	if len(stage.Files) != 5 {
		t.Fatalf("open file size error, expected: 5, actual: %d", len(stage.Files))
	}
}
