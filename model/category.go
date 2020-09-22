package model

import (
	"encoding/json"

	"github.com/ybkuroki/go-webapp-sample/repository"
)

// Category defines struct of category data.
type Category struct {
	ID   uint   `gorm:"primary_key" json:"id"`
	Name string `validate:"required" json:"name"`
}

// TableName returns the table name of category struct and it is used by gorm.
func (Category) TableName() string {
	return "category_master"
}

// NewCategory is constructor
func NewCategory(name string) *Category {
	return &Category{Name: name}
}

// FindByID returns a category full matched given category's ID.
func (c *Category) FindByID(rep *repository.Repository, id uint) (*Category, error) {
	var category Category
	if error := rep.Where("id = ?", id).Find(&category).Error; error != nil {
		return nil, error
	}
	return &category, nil
}

// CountByID returns a size of categories full matched given category's ID.
func (c *Category) CountByID(rep *repository.Repository, id uint) (int, error) {
	var count int
	if error := rep.Model(&Category{}).Where("id = ?", id).Count(&count).Error; error != nil {
		return 0, error
	}
	return count, nil
}

// FindAll returns all categories of the category table.
func (c *Category) FindAll(rep *repository.Repository) (*[]Category, error) {
	var categories []Category
	if error := rep.Find(&categories).Error; error != nil {
		return nil, error
	}
	return &categories, nil
}

// Create persists this category data.
func (c *Category) Create(rep *repository.Repository) (*Category, error) {
	if error := rep.Create(c).Error; error != nil {
		return nil, error
	}
	return c, nil
}

// ToString is return string of object
func (c *Category) ToString() (string, error) {
	bytes, error := json.Marshal(c)
	return string(bytes), error
}
