#!/bin/bash
# Convert Swagger 2.0 YAML fixtures to OpenAPI 3 by simple text replacement

find fixtures -type f \( -name "*.yaml" -o -name "*.yml" \) | while read file; do
    # Check if file contains swagger: "2.0"
    if grep -q 'swagger.*:.*"2\.0"' "$file" || grep -q "swagger.*:.*'2\.0'" "$file" || grep -q 'swagger:.*2\.0' "$file"; then
        echo "Converting $file"

        # Create backup
        cp "$file" "$file.bak"

        # Replace swagger version with openapi version
        sed -i.tmp 's/swagger: *"2\.0"/openapi: "3.0.0"/g' "$file"
        sed -i.tmp "s/swagger: *'2\.0'/openapi: '3.0.0'/g" "$file"
        sed -i.tmp 's/swagger: *2\.0/openapi: "3.0.0"/g' "$file"

        # Convert $ref paths
        sed -i.tmp 's|#/definitions/|#/components/schemas/|g' "$file"
        sed -i.tmp 's|#/parameters/|#/components/parameters/|g' "$file"
        sed -i.tmp 's|#/responses/|#/components/responses/|g' "$file"

        # Convert top-level structures to components
        # This is a simplistic approach - for complex conversions, might need manual review

        # Remove temporary files
        rm -f "$file.tmp"

        echo "  ✓ Converted $file"
    fi
done

echo "Conversion complete!"
