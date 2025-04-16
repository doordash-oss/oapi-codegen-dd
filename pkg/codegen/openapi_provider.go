package codegen

import (
	"fmt"

	"github.com/pb33f/libopenapi"
	"github.com/pb33f/libopenapi/datamodel"
)

func CreateDocument(docContents []byte, cfg Configuration) (libopenapi.Document, error) {
	doc, err := loadDocumentFromContents(docContents)
	if err != nil {
		return nil, err
	}

	doc, err = filterOutDocument(doc, cfg.Filter)
	if err != nil {
		return nil, fmt.Errorf("error filtering document: %w", err)
	}

	if !cfg.SkipPrune {
		doc, err = pruneSchema(doc)
		if err != nil {
			return nil, fmt.Errorf("error pruning schema: %w", err)
		}
	}

	return doc, nil
}

func loadDocumentFromContents(contents []byte) (libopenapi.Document, error) {
	docConfig := &datamodel.DocumentConfiguration{
		SkipCircularReferenceCheck: true,
	}
	doc, err := libopenapi.NewDocumentWithConfiguration(contents, docConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating document: %w", err)
	}
	return doc, nil
}
