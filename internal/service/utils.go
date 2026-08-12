package service

import "fmt"

func ValidateFoodInput(foodmsg CreateFoodInput) error {
	if foodmsg.Name == "" {
		return fmt.Errorf("food name is required")
	}
	return nil
}
