package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type Product struct {
	ProductID    int32  `json:"product_id"`
	SKU          string `json:"sku"`
	Manufacturer string `json:"manufacturer"`
	CategoryID   int32  `json:"category_id"`
	Weight       int32  `json:"weight"`
	SomeOtherID  int32  `json:"some_other_id"`
}

type productInput struct {
	ProductID    *int32  `json:"product_id"`
	SKU          *string `json:"sku"`
	Manufacturer *string `json:"manufacturer"`
	CategoryID   *int32  `json:"category_id"`
	Weight       *int32  `json:"weight"`
	SomeOtherID  *int32  `json:"some_other_id"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

type ProductStore struct {
	mu       sync.RWMutex
	products map[int32]Product
}

func NewProductStore() *ProductStore {
	return &ProductStore{products: make(map[int32]Product)}
}

func (s *ProductStore) Get(id int32) (Product, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.products[id]
	return p, ok
}

func (s *ProductStore) Update(p Product) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.products[p.ProductID]; !ok {
		return false
	}
	s.products[p.ProductID] = p
	return true
}

func (s *ProductStore) Seed(n int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := int32(1); i <= n; i++ {
		s.products[i] = Product{
			ProductID:    i,
			SKU:          fmt.Sprintf("SKU-%05d", i),
			Manufacturer: "Acme Corporation",
			CategoryID:   i%10 + 1,
			Weight:       1000 + i,
			SomeOtherID:  i,
		}
	}
}

const seedCount = 1000

type productHandler struct {
	store *ProductStore
}

func main() {
	store := NewProductStore()
	store.Seed(seedCount)
	h := &productHandler{store: store}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.CustomRecovery(func(c *gin.Context, recovered any) {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "INTERNAL_ERROR",
			Message: "An unexpected error occurred",
			Details: fmt.Sprint(recovered),
		})
	}))
	router.Use(simulateError)

	router.GET("/products/:productId", h.getProduct)
	router.POST("/products/:productId/details", h.addProductDetails)

	router.Run(":8080")
}

func simulateError(c *gin.Context) {
	if c.GetHeader("X-Simulate-Error") == "true" {
		panic("simulated internal error (X-Simulate-Error header)")
	}
	c.Next()
}

func (h *productHandler) getProduct(c *gin.Context) {
	id, err := parseProductID(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Product not found",
			Details: err.Error(),
		})
		return
	}

	p, ok := h.store.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("no product with ID %d", id),
		})
		return
	}

	c.JSON(http.StatusOK, p)
}

func (h *productHandler) addProductDetails(c *gin.Context) {
	id, err := parseProductID(c.Param("productId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: err.Error(),
		})
		return
	}

	var in productInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "Request body must be valid JSON matching the Product schema",
			Details: err.Error(),
		})
		return
	}

	p, err := in.validate(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "INVALID_INPUT",
			Message: "The provided input data is invalid",
			Details: err.Error(),
		})
		return
	}

	if !h.store.Update(p) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "NOT_FOUND",
			Message: "Product not found",
			Details: fmt.Sprintf("no product with ID %d", id),
		})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseProductID(s string) (int32, error) {
	v, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("productId must be a positive integer, got %q", s)
	}
	if v < 1 {
		return 0, fmt.Errorf("productId must be at least 1, got %d", v)
	}
	return int32(v), nil
}

func (in productInput) validate(pathID int32) (Product, error) {
	var missing []string
	if in.ProductID == nil {
		missing = append(missing, "product_id")
	}
	if in.SKU == nil {
		missing = append(missing, "sku")
	}
	if in.Manufacturer == nil {
		missing = append(missing, "manufacturer")
	}
	if in.CategoryID == nil {
		missing = append(missing, "category_id")
	}
	if in.Weight == nil {
		missing = append(missing, "weight")
	}
	if in.SomeOtherID == nil {
		missing = append(missing, "some_other_id")
	}
	if len(missing) > 0 {
		return Product{}, fmt.Errorf("missing required field(s): %s", strings.Join(missing, ", "))
	}

	switch {
	case *in.ProductID < 1:
		return Product{}, errors.New("product_id must be at least 1")
	case *in.ProductID != pathID:
		return Product{}, fmt.Errorf("product_id in body (%d) must match productId in path (%d)", *in.ProductID, pathID)
	case utf8.RuneCountInString(*in.SKU) < 1 || utf8.RuneCountInString(*in.SKU) > 100:
		return Product{}, errors.New("sku must be 1 to 100 characters")
	case utf8.RuneCountInString(*in.Manufacturer) < 1 || utf8.RuneCountInString(*in.Manufacturer) > 200:
		return Product{}, errors.New("manufacturer must be 1 to 200 characters")
	case *in.CategoryID < 1:
		return Product{}, errors.New("category_id must be at least 1")
	case *in.Weight < 0:
		return Product{}, errors.New("weight must be 0 or more")
	case *in.SomeOtherID < 1:
		return Product{}, errors.New("some_other_id must be at least 1")
	}

	return Product{
		ProductID:    *in.ProductID,
		SKU:          *in.SKU,
		Manufacturer: *in.Manufacturer,
		CategoryID:   *in.CategoryID,
		Weight:       *in.Weight,
		SomeOtherID:  *in.SomeOtherID,
	}, nil
}
