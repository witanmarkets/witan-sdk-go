package witan_test

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	witan "github.com/witanmarkets/witan-sdk-go"
)

// Search by the error as it was printed, then read the unit that covers it. No key needed.
func Example() {
	ctx := context.Background()
	c := witan.New()
	a, err := c.SearchError(ctx, "npm error code ENEEDAUTH", nil)
	if err != nil || len(a.Results) == 0 {
		return
	}
	unit, err := c.Read(ctx, a.Results[0].ID)
	if err != nil {
		return
	}
	fmt.Println(unit.Title)
}

// Search a failed command's recent output without stopping the caller if WITAN is unavailable.
func Example_commandFailure() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, "go", "vet", "./...").CombinedOutput()
	if err == nil {
		return
	}
	if len(output) == 0 {
		fmt.Println(err)
		return
	}
	lines := strings.Split(strings.TrimRight(string(output), "\r\n"), "\n")
	if len(lines) > 20 {
		lines = lines[len(lines)-20:]
	}
	answer, err := witan.New().SearchError(ctx, strings.Join(lines, "\n"), nil)
	if err != nil {
		fmt.Println("error search:", err)
		return
	}
	for i, hit := range answer.Results {
		if i == 3 {
			break
		}
		fmt.Printf("%s (%s)\n", hit.Title, hit.ID)
	}
}
