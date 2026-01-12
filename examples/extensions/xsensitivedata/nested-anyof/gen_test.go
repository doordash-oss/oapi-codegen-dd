package nestedanyof

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
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

	// Test 4: UnmarshalJSON for CreditCardPayment
	t.Run("CreditCardPayment_UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"type": "credit_card",
			"cardNumber": "1234-5678-9012-3456",
			"cvv": "123",
			"billingAddress": {
				"street": "123 Main St",
				"city": "New York",
				"zipCode": "10001"
			}
		}`

		var payment CreditCardPayment
		err := json.Unmarshal([]byte(jsonData), &payment)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify type field
		if payment.Type != CreditCard {
			t.Errorf("Expected type to be 'credit_card', got: %v", payment.Type)
		}

		// Verify card number
		if payment.CardNumber != "1234-5678-9012-3456" {
			t.Errorf("Expected cardNumber to be '1234-5678-9012-3456', got: %s", payment.CardNumber)
		}

		// Verify CVV
		if payment.Cvv == nil || *payment.Cvv != "123" {
			t.Errorf("Expected cvv to be '123', got: %v", payment.Cvv)
		}

		// Verify billing address
		if payment.BillingAddress == nil {
			t.Fatal("Expected billingAddress to be present")
		}
		if payment.BillingAddress.Street == nil || *payment.BillingAddress.Street != "123 Main St" {
			t.Errorf("Expected street to be '123 Main St', got: %v", payment.BillingAddress.Street)
		}
		if payment.BillingAddress.City == nil || *payment.BillingAddress.City != "New York" {
			t.Errorf("Expected city to be 'New York', got: %v", payment.BillingAddress.City)
		}
		if payment.BillingAddress.ZipCode == nil || *payment.BillingAddress.ZipCode != "10001" {
			t.Errorf("Expected zipCode to be '10001', got: %v", payment.BillingAddress.ZipCode)
		}
	})

	// Test 5: UnmarshalJSON for DomesticAccount
	t.Run("DomesticAccount_UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"accountType": "domestic",
			"routingNumber": "123456789",
			"accountNumber": "9876543210",
			"accountHolder": {
				"name": "John Doe",
				"email": "john@example.com"
			}
		}`

		var account DomesticAccount
		err := json.Unmarshal([]byte(jsonData), &account)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify accountType field
		if account.AccountType != Domestic {
			t.Errorf("Expected accountType to be 'domestic', got: %v", account.AccountType)
		}

		// Verify routing number
		if account.RoutingNumber != "123456789" {
			t.Errorf("Expected routingNumber to be '123456789', got: %s", account.RoutingNumber)
		}

		// Verify account number
		if account.AccountNumber != "9876543210" {
			t.Errorf("Expected accountNumber to be '9876543210', got: %s", account.AccountNumber)
		}

		// Verify account holder
		if account.AccountHolder == nil {
			t.Fatal("Expected accountHolder to be present")
		}
		if account.AccountHolder.Name == nil || *account.AccountHolder.Name != "John Doe" {
			t.Errorf("Expected name to be 'John Doe', got: %v", account.AccountHolder.Name)
		}
		if account.AccountHolder.Email == nil || *account.AccountHolder.Email != "john@example.com" {
			t.Errorf("Expected email to be 'john@example.com', got: %v", account.AccountHolder.Email)
		}
	})

	// Test 6: UnmarshalJSON for InternationalAccount
	t.Run("InternationalAccount_UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"accountType": "international",
			"iban": "GB82WEST12345698765432",
			"swiftCode": "DEUTDEFF",
			"accountHolder": {
				"name": "Jane Smith",
				"email": "jane@example.com"
			},
			"beneficiaryDetails": {
				"beneficiaryType": "personal",
				"fullName": "Bob Johnson",
				"ssn": "123-45-6789",
				"email": "bob@example.com",
				"phone": "555-123-4567"
			}
		}`

		var account InternationalAccount
		err := json.Unmarshal([]byte(jsonData), &account)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify accountType field
		if account.AccountType != International {
			t.Errorf("Expected accountType to be 'international', got: %v", account.AccountType)
		}

		// Verify IBAN
		if account.Iban != "GB82WEST12345698765432" {
			t.Errorf("Expected iban to be 'GB82WEST12345698765432', got: %s", account.Iban)
		}

		// Verify SWIFT code
		if account.SwiftCode != "DEUTDEFF" {
			t.Errorf("Expected swiftCode to be 'DEUTDEFF', got: %s", account.SwiftCode)
		}

		// Verify account holder
		if account.AccountHolder == nil {
			t.Fatal("Expected accountHolder to be present")
		}
		if account.AccountHolder.Name == nil || *account.AccountHolder.Name != "Jane Smith" {
			t.Errorf("Expected name to be 'Jane Smith', got: %v", account.AccountHolder.Name)
		}
		if account.AccountHolder.Email == nil || *account.AccountHolder.Email != "jane@example.com" {
			t.Errorf("Expected email to be 'jane@example.com', got: %v", account.AccountHolder.Email)
		}

		// Verify beneficiary details
		if account.BeneficiaryDetails == nil {
			t.Fatal("Expected beneficiaryDetails to be present")
		}
		if account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf == nil {
			t.Fatal("Expected beneficiaryDetails anyOf to be present")
		}

		// Verify it's PersonalBeneficiary (type A in Either)
		if !account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsA() {
			t.Fatal("Expected beneficiary to be PersonalBeneficiary (type A)")
		}

		beneficiary := account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.A

		if beneficiary.BeneficiaryType != Personal {
			t.Errorf("Expected beneficiaryType to be 'personal', got: %v", beneficiary.BeneficiaryType)
		}
		if beneficiary.FullName != "Bob Johnson" {
			t.Errorf("Expected fullName to be 'Bob Johnson', got: %s", beneficiary.FullName)
		}
		if beneficiary.Ssn == nil || *beneficiary.Ssn != "123-45-6789" {
			t.Errorf("Expected ssn to be '123-45-6789', got: %v", beneficiary.Ssn)
		}
		if beneficiary.Email == nil || *beneficiary.Email != "bob@example.com" {
			t.Errorf("Expected email to be 'bob@example.com', got: %v", beneficiary.Email)
		}
		if beneficiary.Phone == nil || *beneficiary.Phone != "555-123-4567" {
			t.Errorf("Expected phone to be '555-123-4567', got: %v", beneficiary.Phone)
		}
	})

	// Test 7: UnmarshalJSON for DigitalWalletPayment
	t.Run("DigitalWalletPayment_UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"type": "digital_wallet",
			"walletId": "wallet-12345"
		}`

		var payment DigitalWalletPayment
		err := json.Unmarshal([]byte(jsonData), &payment)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify type field
		if payment.Type != DigitalWallet {
			t.Errorf("Expected type to be 'digital_wallet', got: %v", payment.Type)
		}

		// Verify wallet ID
		if payment.WalletID != "wallet-12345" {
			t.Errorf("Expected walletId to be 'wallet-12345', got: %s", payment.WalletID)
		}
	})

	// Test 8: Round-trip marshal/unmarshal for PaymentMethod with CreditCardPayment
	t.Run("PaymentMethod_CreditCard_RoundTrip", func(t *testing.T) {
		street := "456 Oak Ave"
		city := "Boston"
		zipCode := "02101"
		cvv := "456"

		// Create original payment
		original := PaymentMethod{
			PaymentMethod_AnyOf: &PaymentMethod_AnyOf{},
		}
		creditCard := CreditCardPayment{
			Type:       CreditCard,
			CardNumber: "9876-5432-1098-7654",
			Cvv:        &cvv,
			BillingAddress: &Address{
				Street:  &street,
				City:    &city,
				ZipCode: &zipCode,
			},
		}
		if err := original.PaymentMethod_AnyOf.FromCreditCardPayment(creditCard); err != nil {
			t.Fatalf("Failed to set credit card: %v", err)
		}

		// Marshal
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		t.Logf("Marshaled PaymentMethod: %s", string(data))

		// Unmarshal
		var unmarshaled PaymentMethod
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify unmarshaled data
		if unmarshaled.PaymentMethod_AnyOf == nil {
			t.Fatal("Expected PaymentMethod_AnyOf to be present")
		}

		// Try to get as CreditCardPayment
		result, err := unmarshaled.PaymentMethod_AnyOf.AsCreditCardPayment()
		if err != nil {
			t.Fatalf("Failed to get as CreditCardPayment: %v", err)
		}

		// Note: Marshaled data is masked, so we can't compare exact values
		// Just verify the structure is intact
		if result.Type != CreditCard {
			t.Errorf("Expected type to be 'credit_card', got: %v", result.Type)
		}
		if result.BillingAddress == nil {
			t.Error("Expected billing address to be present")
		}
	})

	// Test 9: Round-trip marshal/unmarshal for BankTransferPayment with nested DomesticAccount
	t.Run("BankTransferPayment_DomesticAccount_RoundTrip", func(t *testing.T) {
		holderName := "Alice Johnson"
		holderEmail := "alice@example.com"

		// Create original payment
		domesticAccount := DomesticAccount{
			AccountType:   Domestic,
			RoutingNumber: "987654321",
			AccountNumber: "1234567890",
			AccountHolder: &AccountHolder{
				Name:  &holderName,
				Email: &holderEmail,
			},
		}

		accountDetailsAnyOf := &BankTransferPayment_AccountDetails_AnyOf{
			Either: runtime.NewEitherFromA[DomesticAccount, InternationalAccount](domesticAccount),
		}

		original := BankTransferPayment{
			Type: BankTransfer,
			AccountDetails: BankTransferPayment_AccountDetails{
				BankTransferPayment_AccountDetails_AnyOf: accountDetailsAnyOf,
			},
		}

		// Marshal
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		t.Logf("Marshaled BankTransferPayment: %s", string(data))

		// Unmarshal
		var unmarshaled BankTransferPayment
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify structure
		if unmarshaled.Type != BankTransfer {
			t.Errorf("Expected type to be 'bank_transfer', got: %v", unmarshaled.Type)
		}

		if unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf == nil {
			t.Fatal("Expected AccountDetails_AnyOf to be present")
		}

		if !unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.IsA() {
			t.Error("Expected account to be DomesticAccount (type A)")
		}

		account := unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.A
		if account.AccountType != Domestic {
			t.Errorf("Expected accountType to be 'domestic', got: %v", account.AccountType)
		}
	})

	// Test 10: Round-trip for deeply nested InternationalAccount with PersonalBeneficiary
	t.Run("InternationalAccount_PersonalBeneficiary_RoundTrip", func(t *testing.T) {
		holderName := "Bob Smith"
		holderEmail := "bob@example.com"
		beneficiarySSN := "987-65-4321"
		beneficiaryEmail := "beneficiary@example.com"
		beneficiaryPhone := "555-987-6543"

		// Create deeply nested structure
		personalBeneficiary := PersonalBeneficiary{
			BeneficiaryType: Personal,
			FullName:        "Charlie Brown",
			Ssn:             &beneficiarySSN,
			Email:           &beneficiaryEmail,
			Phone:           &beneficiaryPhone,
		}

		beneficiaryAnyOf := &InternationalAccount_BeneficiaryDetails_AnyOf{
			Either: runtime.NewEitherFromA[PersonalBeneficiary, BusinessBeneficiary](personalBeneficiary),
		}

		original := InternationalAccount{
			AccountType: International,
			Iban:        "DE89370400440532013000",
			SwiftCode:   "COBADEFF",
			AccountHolder: &AccountHolder{
				Name:  &holderName,
				Email: &holderEmail,
			},
			BeneficiaryDetails: &InternationalAccount_BeneficiaryDetails{
				InternationalAccount_BeneficiaryDetails_AnyOf: beneficiaryAnyOf,
			},
		}

		// Marshal
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		t.Logf("Marshaled InternationalAccount: %s", string(data))

		// Unmarshal
		var unmarshaled InternationalAccount
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify structure
		if unmarshaled.AccountType != International {
			t.Errorf("Expected accountType to be 'international', got: %v", unmarshaled.AccountType)
		}

		if unmarshaled.BeneficiaryDetails == nil {
			t.Fatal("Expected beneficiaryDetails to be present")
		}

		if unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf == nil {
			t.Fatal("Expected beneficiaryDetails anyOf to be present")
		}

		if !unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsA() {
			t.Error("Expected beneficiary to be PersonalBeneficiary (type A)")
		}

		beneficiary := unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.A
		if beneficiary.BeneficiaryType != Personal {
			t.Errorf("Expected beneficiaryType to be 'personal', got: %v", beneficiary.BeneficiaryType)
		}
		if beneficiary.FullName != "Charlie Brown" {
			t.Errorf("Expected fullName to be 'Charlie Brown', got: %s", beneficiary.FullName)
		}
	})

	// Test 11: Round-trip for PaymentMethod with nested unions (BankTransferPayment -> InternationalAccount -> BusinessBeneficiary)
	t.Run("PaymentMethod_BankTransfer_International_Business_RoundTrip", func(t *testing.T) {
		holderName := "Corp Account"
		holderEmail := "corp@example.com"
		taxID := "12-3456789"

		// Create deeply nested structure: PaymentMethod -> BankTransferPayment -> InternationalAccount -> BusinessBeneficiary
		businessBeneficiary := BusinessBeneficiary{
			BeneficiaryType: Business,
			CompanyName:     "Acme Corp",
			TaxID:           &taxID,
		}

		beneficiaryAnyOf := &InternationalAccount_BeneficiaryDetails_AnyOf{
			Either: runtime.NewEitherFromB[PersonalBeneficiary, BusinessBeneficiary](businessBeneficiary),
		}

		internationalAccount := InternationalAccount{
			AccountType: International,
			Iban:        "FR1420041010050500013M02606",
			SwiftCode:   "BNPAFRPP",
			AccountHolder: &AccountHolder{
				Name:  &holderName,
				Email: &holderEmail,
			},
			BeneficiaryDetails: &InternationalAccount_BeneficiaryDetails{
				InternationalAccount_BeneficiaryDetails_AnyOf: beneficiaryAnyOf,
			},
		}

		accountDetailsAnyOf := &BankTransferPayment_AccountDetails_AnyOf{
			Either: runtime.NewEitherFromB[DomesticAccount, InternationalAccount](internationalAccount),
		}

		bankTransfer := BankTransferPayment{
			Type: BankTransfer,
			AccountDetails: BankTransferPayment_AccountDetails{
				BankTransferPayment_AccountDetails_AnyOf: accountDetailsAnyOf,
			},
		}

		original := PaymentMethod{
			PaymentMethod_AnyOf: &PaymentMethod_AnyOf{},
		}
		if err := original.PaymentMethod_AnyOf.FromBankTransferPayment(bankTransfer); err != nil {
			t.Fatalf("Failed to set bank transfer: %v", err)
		}

		// Marshal
		data, err := json.Marshal(original)
		if err != nil {
			t.Fatalf("Failed to marshal: %v", err)
		}

		t.Logf("Marshaled nested PaymentMethod: %s", string(data))

		// Unmarshal
		var unmarshaled PaymentMethod
		err = json.Unmarshal(data, &unmarshaled)
		if err != nil {
			t.Fatalf("Failed to unmarshal: %v", err)
		}

		// Verify deeply nested structure
		if unmarshaled.PaymentMethod_AnyOf == nil {
			t.Fatal("Expected PaymentMethod_AnyOf to be present")
		}

		bankTransferResult, err := unmarshaled.PaymentMethod_AnyOf.AsBankTransferPayment()
		if err != nil {
			t.Fatalf("Failed to get as BankTransferPayment: %v", err)
		}

		if bankTransferResult.Type != BankTransfer {
			t.Errorf("Expected type to be 'bank_transfer', got: %v", bankTransferResult.Type)
		}

		if bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf == nil {
			t.Fatal("Expected AccountDetails_AnyOf to be present")
		}

		if !bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.IsB() {
			t.Fatal("Expected account to be InternationalAccount (type B)")
		}

		intlAccount := bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.B
		if intlAccount.AccountType != International {
			t.Errorf("Expected accountType to be 'international', got: %v", intlAccount.AccountType)
		}

		if intlAccount.BeneficiaryDetails == nil {
			t.Fatal("Expected beneficiaryDetails to be present")
		}

		if !intlAccount.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsB() {
			t.Fatal("Expected beneficiary to be BusinessBeneficiary (type B)")
		}

		businessResult := intlAccount.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.B
		if businessResult.BeneficiaryType != Business {
			t.Errorf("Expected beneficiaryType to be 'business', got: %v", businessResult.BeneficiaryType)
		}
		if businessResult.CompanyName != "Acme Corp" {
			t.Errorf("Expected companyName to be 'Acme Corp', got: %s", businessResult.CompanyName)
		}
	})
}
