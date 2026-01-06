package nestedanyof

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/doordash/oapi-codegen-dd/v3/pkg/runtime"
)

func TestNestedAnyOfWithSensitiveData(t *testing.T) {
	// Test 1: Credit card payment with billing address
	t.Run("CreditCardPayment", func(t *testing.T) {
		street := "123 Main St"
		city := "New York"
		zipCode := "10001"
		cvv := "123"

		payment := CreditCardPayment{
			Type:       CreditCard,
			CardNumber: "1234-5678-9012-3456",
			Cvv:        &cvv,
			BillingAddress: &Address{
				Street:  &street,
				City:    &city,
				ZipCode: &zipCode,
			},
		}

		data, err := json.Marshal(payment)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		jsonStr := string(data)
		t.Logf("CreditCardPayment JSON: %s", jsonStr)

		// Verify card number shows last 4 digits
		if !strings.Contains(jsonStr, `"cardNumber":"********3456"`) {
			t.Errorf("Card number should show last 4 digits, got: %s", jsonStr)
		}

		// Verify CVV is fully masked
		if !strings.Contains(jsonStr, `"cvv":"********"`) {
			t.Errorf("CVV should be fully masked, got: %s", jsonStr)
		}

		// Verify billing address is not masked
		if !strings.Contains(jsonStr, `"street":"123 Main St"`) {
			t.Errorf("Street should not be masked, got: %s", jsonStr)
		}
	})

	// Test 2: Domestic bank account (nested in BankTransferPayment)
	t.Run("BankTransferPayment_DomesticAccount", func(t *testing.T) {
		holderName := "John Doe"
		holderEmail := "john@example.com"

		domesticAccount := DomesticAccount{
			AccountType:   Domestic,
			RoutingNumber: "123456789",
			AccountNumber: "9876543210",
			AccountHolder: &AccountHolder{
				Name:  &holderName,
				Email: &holderEmail,
			},
		}

		data, err := json.Marshal(domesticAccount)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		jsonStr := string(data)
		t.Logf("DomesticAccount JSON: %s", jsonStr)

		// Verify routing number shows first 2 and last 2
		if !strings.Contains(jsonStr, `"routingNumber":"12********89"`) {
			t.Errorf("Routing number should show first 2 and last 2, got: %s", jsonStr)
		}

		// Verify account number shows last 4
		if !strings.Contains(jsonStr, `"accountNumber":"********3210"`) {
			t.Errorf("Account number should show last 4, got: %s", jsonStr)
		}

		// Verify account holder email is masked
		if !strings.Contains(jsonStr, `"email":"********"`) {
			t.Errorf("Email should be fully masked, got: %s", jsonStr)
		}
	})

	// Test 3: International account with personal beneficiary (deeply nested)
	t.Run("InternationalAccount_PersonalBeneficiary", func(t *testing.T) {
		holderName := "Jane Smith"
		holderEmail := "jane@example.com"
		beneficiarySSN := "123-45-6789"
		beneficiaryEmail := "bob@example.com"
		beneficiaryPhone := "555-123-4567"

		personalBeneficiary := PersonalBeneficiary{
			BeneficiaryType: Personal,
			FullName:        "Bob Johnson",
			Ssn:             &beneficiarySSN,
			Email:           &beneficiaryEmail,
			Phone:           &beneficiaryPhone,
		}

		// Create the anyOf wrapper
		beneficiaryAnyOf := &InternationalAccount_BeneficiaryDetails_AnyOf{
			Either: runtime.NewEitherFromA[PersonalBeneficiary, BusinessBeneficiary](personalBeneficiary),
		}

		internationalAccount := InternationalAccount{
			AccountType: International,
			Iban:        "GB82WEST12345698765432",
			SwiftCode:   "DEUTDEFF",
			AccountHolder: &AccountHolder{
				Name:  &holderName,
				Email: &holderEmail,
			},
			BeneficiaryDetails: &InternationalAccount_BeneficiaryDetails{
				InternationalAccount_BeneficiaryDetails_AnyOf: beneficiaryAnyOf,
			},
		}

		data, err := json.Marshal(internationalAccount)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		jsonStr := string(data)
		t.Logf("InternationalAccount with PersonalBeneficiary JSON: %s", jsonStr)

		// Verify IBAN shows first 4 and last 4
		if !strings.Contains(jsonStr, `"iban":"GB82********5432"`) {
			t.Errorf("IBAN should show first 4 and last 4, got: %s", jsonStr)
		}

		// Verify SWIFT code is fully masked
		if !strings.Contains(jsonStr, `"swiftCode":"********"`) {
			t.Errorf("SWIFT code should be fully masked, got: %s", jsonStr)
		}

		// Verify account holder email is masked
		if strings.Count(jsonStr, `"email":"********"`) < 2 {
			t.Errorf("Both emails should be fully masked, got: %s", jsonStr)
		}

		// Verify SSN has all digits masked (regex pattern)
		if !strings.Contains(jsonStr, `"ssn":"***-**-****"`) {
			t.Errorf("SSN should have all digits masked, got: %s", jsonStr)
		}

		// Verify phone shows first 3 and last 4
		if !strings.Contains(jsonStr, `"phone":"555********4567"`) {
			t.Errorf("Phone should show first 3 and last 4, got: %s", jsonStr)
		}
	})
}
