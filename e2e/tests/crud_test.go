package tests

import (
	"fmt"
	"net/http"
	"testing"

	"gateway-e2e/internal/client"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exemplar for STATEFUL testing. Key idea: isolate each test by resetting the
// backend first, so order and reruns never matter.
//
// Uses the single-backend /data route on purpose — a load-balanced route would
// scatter state across replicas. Covers test points §9.

// freshData returns an authed client after clearing the data backend.
func freshData(t *testing.T) *client.Client {
	t.Helper()
	c := authClient(1)
	resp, err := c.Post("/data/_reset", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)
	return c
}

// §9.1, §9.2, §9.4 — full create → get → delete → 404 lifecycle.
func TestCRUD_Lifecycle(t *testing.T) {
	c := freshData(t)

	// create
	resp, err := c.Post("/data/items", map[string]any{"name": "alpha"})
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, resp.Status)

	var created struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	require.NoError(t, resp.JSON(&created))
	assert.Equal(t, "alpha", created.Name)
	require.NotZero(t, created.ID)

	// get it back
	resp, err = c.Get(fmt.Sprintf("/data/items/%d", created.ID))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.Status)

	// delete
	resp, err = c.Delete(fmt.Sprintf("/data/items/%d", created.ID))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.Status)

	// gone now
	resp, err = c.Get(fmt.Sprintf("/data/items/%d", created.ID))
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.Status)
}

// §9.8 — reset isolates tests: a fresh backend lists zero items.
func TestCRUD_ResetIsolatesState(t *testing.T) {
	c := freshData(t)

	resp, err := c.Get("/data/items")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Status)

	var list struct {
		Count int `json:"count"`
	}
	require.NoError(t, resp.JSON(&list))
	assert.Equal(t, 0, list.Count, "reset should leave the store empty")
}

// §9.5 — deleting a non-existent item is a clean 404.
func TestCRUD_DeleteMissing(t *testing.T) {
	c := freshData(t)
	resp, err := c.Delete("/data/items/999999")
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.Status)
}
