package service

import (
	"fmt"
	"math"
)

func validateFoodInput(foodmsg CreateFoodInput) error {
	if err := validateTextLength("food_name", foodmsg.Name, 1, 128); err != nil {
		return err
	}
	if err := validateNutritionNumber("calories_per_100g", foodmsg.CaloriesPer100G, 0, 1000); err != nil {
		return err
	}
	if err := validateOptionalNutritionNumber("carbohydrate_per_100g", foodmsg.CarbohydratePer100G, 0, 100); err != nil {
		return err
	}

	if err := validateOptionalNutritionNumber("protein_per_100g", foodmsg.ProteinPer100G, 0, 100); err != nil {
		return err
	}

	if err := validateOptionalNutritionNumber("fat_per_100g", foodmsg.FatPer100G, 0, 100); err != nil {
		return err
	}

	if !foodmsg.IconType.Valid() {
		return fmt.Errorf(
			"%w: invalid food icon type",
			ErrInvalidInput,
		)
	}

	if err := validateTextLength("icon_value", foodmsg.IconValue, 1, 255); err != nil {
		return err
	}
	return nil
}

func validateNutritionNumber(field string, value float64, minium float64, maxium float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return fmt.Errorf("%w: %s must be a finite number", ErrInvalidInput, field)
	}
	if value < minium || value > maxium {
		return fmt.Errorf(
			"%w: %s must be between %.0f and %.0f",
			ErrInvalidInput,
			field,
			minium,
			maxium,
		)
	}
	return nil
}

func validateOptionalNutritionNumber(field string, value *float64, minium float64, maxium float64) error {
	if value == nil {
		return nil
	}
	return validateNutritionNumber(field, *value, minium, maxium)
}

func validateUpdateFoodInput(foodmsg UpdateFoodInput) error {
	if foodmsg.Version == 0 {
		return fmt.Errorf("%w: version must be greater than 0", ErrInvalidInput)
	}
	return validateFoodInput(CreateFoodInput{
		Name:                     foodmsg.Name,
		CaloriesPer100G:          foodmsg.CaloriesPer100G,
		CarbohydratePer100G:      foodmsg.CarbohydratePer100G,
		ProteinPer100G:           foodmsg.ProteinPer100G,
		FatPer100G:               foodmsg.FatPer100G,
		IconType:                 foodmsg.IconType,
		IconValue:                foodmsg.IconValue,
		ConfirmNutritionMismatch: foodmsg.ConfirmNutritionMismatch,
	})
}
