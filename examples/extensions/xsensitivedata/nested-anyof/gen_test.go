package nestedanyof

import (
	"encoding/json"
	"testing"

	"github.com/doordash-oss/oapi-codegen-dd/v3/pkg/runtime"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNestedAnyOfWithSensitiveData(t *testing.T) {
	// Test 1: Credit card payment with billing address - masked output
	t.Run("CreditCardPayment_Masked", func(t *testing.T) {
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

		// Use Masked() to get masked output
		data, err := json.Marshal(payment.Masked())
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify card number shows last 4 digits
		assert.Contains(t, jsonStr, `"cardNumber":"********3456"`, "Card number should show last 4 digits")

		// Verify CVV is fully masked
		assert.Contains(t, jsonStr, `"cvv":"********"`, "CVV should be fully masked")

		// Verify billing address is not masked
		assert.Contains(t, jsonStr, `"street":"123 Main St"`, "Street should not be masked")
	})

	// Test 2: Domestic bank account (nested in BankTransferPayment) - masked output
	t.Run("BankTransferPayment_DomesticAccount_Masked", func(t *testing.T) {
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

		// Use Masked() to get masked output
		data, err := json.Marshal(domesticAccount.Masked())
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify routing number shows first 2 and last 2
		assert.Contains(t, jsonStr, `"routingNumber":"12********89"`, "Routing number should show first 2 and last 2")

		// Verify account number shows last 4
		assert.Contains(t, jsonStr, `"accountNumber":"********3210"`, "Account number should show last 4")

		// Note: AccountHolder is a nested struct, Masked() only masks direct fields
		// AccountHolder.Email is not masked by DomesticAccount.Masked()
	})

	// Test 3: International account with personal beneficiary - masked output
	t.Run("InternationalAccount_PersonalBeneficiary_Masked", func(t *testing.T) {
		internationalAccount := InternationalAccount{
			AccountType: International,
			Iban:        "GB82WEST12345698765432",
			SwiftCode:   "DEUTDEFF",
		}

		// Use Masked() to get masked output
		data, err := json.Marshal(internationalAccount.Masked())
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify IBAN shows first 4 and last 4
		assert.Contains(t, jsonStr, `"iban":"GB82********5432"`, "IBAN should show first 4 and last 4")

		// Verify SWIFT code is fully masked
		assert.Contains(t, jsonStr, `"swiftCode":"********"`, "SWIFT code should be fully masked")
	})

	// Test 3b: PersonalBeneficiary masked output
	t.Run("PersonalBeneficiary_Masked", func(t *testing.T) {
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

		// Use Masked() to get masked output
		data, err := json.Marshal(personalBeneficiary.Masked())
		require.NoError(t, err)

		jsonStr := string(data)

		// Verify email is fully masked
		assert.Contains(t, jsonStr, `"email":"********"`, "Email should be fully masked")

		// Verify SSN has all digits masked (regex pattern)
		assert.Contains(t, jsonStr, `"ssn":"***-**-****"`, "SSN should have all digits masked")

		// Verify phone shows first 3 and last 4
		assert.Contains(t, jsonStr, `"phone":"555********4567"`, "Phone should show first 3 and last 4")
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
		require.NoError(t, json.Unmarshal([]byte(jsonData), &payment))

		assert.Equal(t, CreditCard, payment.Type)
		assert.Equal(t, "1234-5678-9012-3456", payment.CardNumber)
		require.NotNil(t, payment.Cvv)
		assert.Equal(t, "123", *payment.Cvv)

		require.NotNil(t, payment.BillingAddress)
		require.NotNil(t, payment.BillingAddress.Street)
		assert.Equal(t, "123 Main St", *payment.BillingAddress.Street)
		require.NotNil(t, payment.BillingAddress.City)
		assert.Equal(t, "New York", *payment.BillingAddress.City)
		require.NotNil(t, payment.BillingAddress.ZipCode)
		assert.Equal(t, "10001", *payment.BillingAddress.ZipCode)
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
		require.NoError(t, json.Unmarshal([]byte(jsonData), &account))

		assert.Equal(t, Domestic, account.AccountType)
		assert.Equal(t, "123456789", account.RoutingNumber)
		assert.Equal(t, "9876543210", account.AccountNumber)

		require.NotNil(t, account.AccountHolder)
		require.NotNil(t, account.AccountHolder.Name)
		assert.Equal(t, "John Doe", *account.AccountHolder.Name)
		require.NotNil(t, account.AccountHolder.Email)
		assert.Equal(t, "john@example.com", *account.AccountHolder.Email)
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
		require.NoError(t, json.Unmarshal([]byte(jsonData), &account))

		assert.Equal(t, International, account.AccountType)
		assert.Equal(t, "GB82WEST12345698765432", account.Iban)
		assert.Equal(t, "DEUTDEFF", account.SwiftCode)

		require.NotNil(t, account.AccountHolder)
		require.NotNil(t, account.AccountHolder.Name)
		assert.Equal(t, "Jane Smith", *account.AccountHolder.Name)
		require.NotNil(t, account.AccountHolder.Email)
		assert.Equal(t, "jane@example.com", *account.AccountHolder.Email)

		require.NotNil(t, account.BeneficiaryDetails)
		require.NotNil(t, account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf)
		require.True(t, account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsA(), "Expected PersonalBeneficiary (type A)")

		beneficiary := account.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.A
		assert.Equal(t, Personal, beneficiary.BeneficiaryType)
		assert.Equal(t, "Bob Johnson", beneficiary.FullName)
		require.NotNil(t, beneficiary.Ssn)
		assert.Equal(t, "123-45-6789", *beneficiary.Ssn)
		require.NotNil(t, beneficiary.Email)
		assert.Equal(t, "bob@example.com", *beneficiary.Email)
		require.NotNil(t, beneficiary.Phone)
		assert.Equal(t, "555-123-4567", *beneficiary.Phone)
	})

	// Test 7: UnmarshalJSON for DigitalWalletPayment
	t.Run("DigitalWalletPayment_UnmarshalJSON", func(t *testing.T) {
		jsonData := `{
			"type": "digital_wallet",
			"walletId": "wallet-12345"
		}`

		var payment DigitalWalletPayment
		require.NoError(t, json.Unmarshal([]byte(jsonData), &payment))

		assert.Equal(t, DigitalWallet, payment.Type)
		assert.Equal(t, "wallet-12345", payment.WalletID)
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
		require.NoError(t, original.PaymentMethod_AnyOf.FromCreditCardPayment(creditCard))

		// Marshal
		data, err := json.Marshal(original)
		require.NoError(t, err)

		// Unmarshal
		var unmarshaled PaymentMethod
		require.NoError(t, json.Unmarshal(data, &unmarshaled))

		require.NotNil(t, unmarshaled.PaymentMethod_AnyOf)

		result, err := unmarshaled.PaymentMethod_AnyOf.AsCreditCardPayment()
		require.NoError(t, err)

		assert.Equal(t, CreditCard, result.Type)
		assert.NotNil(t, result.BillingAddress)
	})

	// Test 9: Round-trip marshal/unmarshal for BankTransferPayment with nested DomesticAccount
	t.Run("BankTransferPayment_DomesticAccount_RoundTrip", func(t *testing.T) {
		holderName := "Alice Johnson"
		holderEmail := "alice@example.com"

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

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled BankTransferPayment
		require.NoError(t, json.Unmarshal(data, &unmarshaled))

		assert.Equal(t, BankTransfer, unmarshaled.Type)
		require.NotNil(t, unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf)
		require.True(t, unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.IsA(), "Expected DomesticAccount (type A)")

		account := unmarshaled.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.A
		assert.Equal(t, Domestic, account.AccountType)
	})

	// Test 10: Round-trip for deeply nested InternationalAccount with PersonalBeneficiary
	t.Run("InternationalAccount_PersonalBeneficiary_RoundTrip", func(t *testing.T) {
		holderName := "Bob Smith"
		holderEmail := "bob@example.com"
		beneficiarySSN := "987-65-4321"
		beneficiaryEmail := "beneficiary@example.com"
		beneficiaryPhone := "555-987-6543"

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

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled InternationalAccount
		require.NoError(t, json.Unmarshal(data, &unmarshaled))

		assert.Equal(t, International, unmarshaled.AccountType)
		require.NotNil(t, unmarshaled.BeneficiaryDetails)
		require.NotNil(t, unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf)
		require.True(t, unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsA(), "Expected PersonalBeneficiary (type A)")

		beneficiary := unmarshaled.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.A
		assert.Equal(t, Personal, beneficiary.BeneficiaryType)
		assert.Equal(t, "Charlie Brown", beneficiary.FullName)
	})

	// Test 11: Round-trip for PaymentMethod with nested unions (BankTransferPayment -> InternationalAccount -> BusinessBeneficiary)
	t.Run("PaymentMethod_BankTransfer_International_Business_RoundTrip", func(t *testing.T) {
		holderName := "Corp Account"
		holderEmail := "corp@example.com"
		taxID := "12-3456789"

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
		require.NoError(t, original.PaymentMethod_AnyOf.FromBankTransferPayment(bankTransfer))

		data, err := json.Marshal(original)
		require.NoError(t, err)

		var unmarshaled PaymentMethod
		require.NoError(t, json.Unmarshal(data, &unmarshaled))

		require.NotNil(t, unmarshaled.PaymentMethod_AnyOf)

		bankTransferResult, err := unmarshaled.PaymentMethod_AnyOf.AsBankTransferPayment()
		require.NoError(t, err)

		assert.Equal(t, BankTransfer, bankTransferResult.Type)
		require.NotNil(t, bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf)
		require.True(t, bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.IsB(), "Expected InternationalAccount (type B)")

		intlAccount := bankTransferResult.AccountDetails.BankTransferPayment_AccountDetails_AnyOf.B
		assert.Equal(t, International, intlAccount.AccountType)

		require.NotNil(t, intlAccount.BeneficiaryDetails)
		require.True(t, intlAccount.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.IsB(), "Expected BusinessBeneficiary (type B)")

		businessResult := intlAccount.BeneficiaryDetails.InternationalAccount_BeneficiaryDetails_AnyOf.B
		assert.Equal(t, Business, businessResult.BeneficiaryType)
		assert.Equal(t, "Acme Corp", businessResult.CompanyName)
	})
}
