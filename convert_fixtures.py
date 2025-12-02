#!/usr/bin/env python3
"""
Convert Swagger 2.0 fixture files to OpenAPI 3.0 format.
This script handles the structural changes needed for the migration.
"""

import os
import sys
import json
import yaml
from pathlib import Path


def convert_ref(ref_string):
    """Convert Swagger 2.0 $ref paths to OpenAPI 3 paths."""
    if not ref_string:
        return ref_string

    # Convert internal references
    ref_string = ref_string.replace('#/definitions/', '#/components/schemas/')
    ref_string = ref_string.replace('#/parameters/', '#/components/parameters/')
    ref_string = ref_string.replace('#/responses/', '#/components/responses/')

    return ref_string


def convert_schema_refs(obj):
    """Recursively convert all $ref in an object."""
    if isinstance(obj, dict):
        if '$ref' in obj:
            obj['$ref'] = convert_ref(obj['$ref'])
        for key, value in obj.items():
            convert_schema_refs(value)
    elif isinstance(obj, list):
        for item in obj:
            convert_schema_refs(item)


def convert_spec(spec):
    """Convert a Swagger 2.0 spec to OpenAPI 3.0."""
    if not isinstance(spec, dict):
        return spec

    # Skip if already OpenAPI 3
    if 'openapi' in spec:
        return spec

    # Skip if not a Swagger spec
    if 'swagger' not in spec:
        return spec

    # Create new OpenAPI 3 structure
    new_spec = {
        'openapi': '3.0.0',
        'info': spec.get('info', {'title': 'API', 'version': '1.0.0'})
    }

    # Copy basic fields
    if 'externalDocs' in spec:
        new_spec['externalDocs'] = spec['externalDocs']

    if 'tags' in spec:
        new_spec['tags'] = spec['tags']

    if 'security' in spec:
        new_spec['security'] = spec['security']

    # Convert servers from host/basePath/schemes
    servers = []
    if 'host' in spec or 'basePath' in spec or 'schemes' in spec:
        schemes = spec.get('schemes', ['http'])
        host = spec.get('host', 'localhost')
        base_path = spec.get('basePath', '')
        for scheme in schemes:
            servers.append({'url': f'{scheme}://{host}{base_path}'})

    if servers:
        new_spec['servers'] = servers

    # Initialize components
    components = {}

    # Convert definitions to schemas
    if 'definitions' in spec:
        components['schemas'] = spec['definitions']

    # Convert parameters
    if 'parameters' in spec:
        components['parameters'] = spec['parameters']

    # Convert responses
    if 'responses' in spec:
        components['responses'] = spec['responses']

    # Convert securityDefinitions to securitySchemes
    if 'securityDefinitions' in spec:
        components['securitySchemes'] = spec['securityDefinitions']

    if components:
        new_spec['components'] = components

    # Convert paths
    if 'paths' in spec:
        new_spec['paths'] = spec['paths']

    # Copy extensions
    for key in spec:
        if key.startswith('x-'):
            new_spec[key] = spec[key]

    # Convert all $refs in the spec
    convert_schema_refs(new_spec)

    return new_spec


def convert_file(filepath):
    """Convert a single fixture file."""
    try:
        # Read the file
        with open(filepath, 'r', encoding='utf-8') as f:
            if filepath.suffix == '.json':
                data = json.load(f)
            else:
                data = yaml.safe_load(f)

        if not data:
            return False

        # Convert the spec
        converted = convert_spec(data)

        # Check if anything changed
        if converted == data:
            return False

        # Write back
        with open(filepath, 'w', encoding='utf-8') as f:
            if filepath.suffix == '.json':
                json.dump(converted, f, indent=2, ensure_ascii=False)
                f.write('\n')
            else:
                yaml.dump(converted, f, default_flow_style=False, allow_unicode=True, sort_keys=False)

        return True

    except Exception as e:
        print(f"Error converting {filepath}: {e}", file=sys.stderr)
        return False


def main():
    """Convert all fixture files in the fixtures directory."""
    fixtures_dir = Path(__file__).parent / 'fixtures'

    if not fixtures_dir.exists():
        print(f"Fixtures directory not found: {fixtures_dir}", file=sys.stderr)
        return 1

    # Find all YAML and JSON files
    patterns = ['**/*.yaml', '**/*.yml', '**/*.json']
    files = []
    for pattern in patterns:
        files.extend(fixtures_dir.glob(pattern))

    print(f"Found {len(files)} fixture files to process")

    converted_count = 0
    for filepath in sorted(files):
        if convert_file(filepath):
            converted_count += 1
            print(f"Converted: {filepath.relative_to(fixtures_dir)}")

    print(f"\nConverted {converted_count} out of {len(files)} files")
    return 0


if __name__ == '__main__':
    sys.exit(main())
