package witan_test

import (
	"context"
	"fmt"

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
