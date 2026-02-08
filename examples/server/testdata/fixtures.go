// Package testdata provides shared test fixtures for server examples.
package testdata

import (
	"fmt"
	"time"
)

// User represents a user fixture.
type User struct {
	ID    string
	Name  string
	Email string
}

// Product represents a product fixture.
type Product struct {
	ID    string
	Name  string
	Price float32
	Tags  []string
}

// Post represents a post fixture.
type Post struct {
	ID      string
	UserID  string
	Title   string
	Content string
}

// Company represents a company fixture.
type Company struct {
	ID      string
	Name    string
	Street  string
	City    string
	Country string
}

// Contact represents a contact fixture.
type Contact struct {
	Name  string
	Email string
	Phone string
}

// Users returns sample user fixtures.
func Users() []User {
	return []User{
		{ID: "1", Name: "Alice", Email: "alice@example.com"},
		{ID: "2", Name: "Bob", Email: "bob@example.com"},
		{ID: "3", Name: "Charlie", Email: "charlie@example.com"},
	}
}

// Products returns sample product fixtures.
func Products() []Product {
	return []Product{
		{ID: "prod-1", Name: "Laptop", Price: 999.99, Tags: []string{"electronics", "computers"}},
		{ID: "prod-2", Name: "Phone", Price: 599.99, Tags: []string{"electronics", "mobile"}},
		{ID: "prod-3", Name: "Headphones", Price: 199.99, Tags: []string{"electronics", "audio"}},
	}
}

// FilterProductsByIDs filters products by IDs.
func FilterProductsByIDs(products []Product, ids []string) []Product {
	if len(ids) == 0 {
		return products
	}
	filtered := []Product{}
	for _, p := range products {
		for _, id := range ids {
			if p.ID == id {
				filtered = append(filtered, p)
				break
			}
		}
	}
	return filtered
}

// FilterProductsByTags filters products by tags.
func FilterProductsByTags(products []Product, tags []string) []Product {
	if len(tags) == 0 {
		return products
	}
	filtered := []Product{}
	for _, p := range products {
		for _, tag := range tags {
			for _, pTag := range p.Tags {
				if pTag == tag {
					filtered = append(filtered, p)
					goto next
				}
			}
		}
	next:
	}
	return filtered
}

// NewPost creates a post fixture for the given user and post IDs.
func NewPost(userID, postID string) Post {
	return Post{
		ID:      postID,
		UserID:  userID,
		Title:   fmt.Sprintf("Post %s by User %s", postID, userID),
		Content: "This is the post content.",
	}
}

// NewOrderID generates a unique order ID.
func NewOrderID() string {
	return fmt.Sprintf("order-%d", time.Now().UnixNano())
}

// NewCompanyID generates a unique company ID.
func NewCompanyID() string {
	return fmt.Sprintf("company-%d", time.Now().UnixNano())
}
