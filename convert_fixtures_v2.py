#!/usr/bin/env python3
"""
Convert OpenAPI v2 (Swagger 2.0) fixtures to OpenAPI v3.0.0
"""

import json
import os
import sys
from pathlib import Path
from typing import Any, Dict, Optional


def convert_server(host: str, base_path: str, schemes: list) -> list:
    """Convert host, basePath, and schemes to servers array"""
    servers = []

    if not host:
        return servers

    # Use schemes if provided, otherwise default to https
    schemes_list = schemes if schemes else ['https']
    base = base_path if base_path else ''

    for scheme in schemes_list:
        url = f"{scheme}://{host}{base}"
        servers.append({"url": url})

    return servers


def convert_parameter(param: Dict[str, Any]) -> Dict[str, Any]:
    """Convert v2 parameter to v3 parameter"""
    new_param = {}

    # Copy basic fields
    for field in ['name', 'in', 'description', 'required', 'deprecated']:
        if field in param:
            new_param[field] = param[field]

    # Handle body parameters - convert to requestBody (handled at operation level)
    if param.get('in') == 'body':
        return None  # Signal that this needs requestBody conversion

    # Handle form parameters - convert to requestBody (handled at operation level)
    if param.get('in') == 'formData':
        return None  # Signal that this needs requestBody conversion

    # Convert schema fields to schema object for non-body parameters
    schema_fields = ['type', 'format', 'items', 'collectionFormat', 'default',
                     'maximum', 'minimum', 'maxLength', 'minLength', 'pattern',
                     'maxItems', 'minItems', 'uniqueItems', 'enum', 'multipleOf']

    schema = {}
    for field in schema_fields:
        if field in param:
            schema[field] = param[field]

    if schema:
        new_param['schema'] = schema

    # Handle examples
    if 'x-example' in param:
        new_param['example'] = param['x-example']

    return new_param


def convert_response(response: Dict[str, Any]) -> Dict[str, Any]:
    """Convert v2 response to v3 response"""
    new_response = {}

    # Copy description (required)
    if 'description' in response:
        new_response['description'] = response['description']
    else:
        new_response['description'] = ''

    # Convert schema to content
    if 'schema' in response:
        new_response['content'] = {
            'application/json': {
                'schema': response['schema']
            }
        }

    # Handle examples - convert to content examples
    if 'examples' in response and 'content' in new_response:
        # v2 examples is a map of mime types to examples
        for mime_type, example in response['examples'].items():
            if mime_type not in new_response['content']:
                new_response['content'][mime_type] = {}
            if 'examples' not in new_response['content'][mime_type]:
                new_response['content'][mime_type]['examples'] = {}
            new_response['content'][mime_type]['examples']['example'] = {
                'value': example
            }

    # Copy headers
    if 'headers' in response:
        new_response['headers'] = response['headers']

    return new_response


def convert_operation(operation: Dict[str, Any]) -> Dict[str, Any]:
    """Convert v2 operation to v3 operation"""
    new_op = {}

    # Copy basic fields
    for field in ['tags', 'summary', 'description', 'operationId', 'deprecated',
                  'security', 'externalDocs']:
        if field in operation:
            new_op[field] = operation[field]

    # Handle parameters
    if 'parameters' in operation:
        new_params = []
        body_params = []
        form_params = []

        for param in operation['parameters']:
            if isinstance(param, dict):
                if param.get('in') == 'body':
                    body_params.append(param)
                elif param.get('in') == 'formData':
                    form_params.append(param)
                else:
                    converted = convert_parameter(param)
                    if converted:
                        new_params.append(converted)
            else:
                # It's a $ref
                new_params.append(param)

        if new_params:
            new_op['parameters'] = new_params

        # Convert body parameters to requestBody
        if body_params:
            body_param = body_params[0]  # Usually only one body param
            new_op['requestBody'] = {
                'required': body_param.get('required', False),
                'content': {
                    'application/json': {
                        'schema': body_param.get('schema', {})
                    }
                }
            }
            if 'description' in body_param:
                new_op['requestBody']['description'] = body_param['description']

        # Convert formData parameters to requestBody
        if form_params:
            properties = {}
            required = []
            for param in form_params:
                param_name = param['name']
                properties[param_name] = {
                    'type': param.get('type', 'string')
                }
                for field in ['format', 'description', 'default', 'enum']:
                    if field in param:
                        properties[param_name][field] = param[field]

                if param.get('required'):
                    required.append(param_name)

            new_op['requestBody'] = {
                'required': bool(required),
                'content': {
                    'application/x-www-form-urlencoded': {
                        'schema': {
                            'type': 'object',
                            'properties': properties
                        }
                    }
                }
            }
            if required:
                new_op['requestBody']['content']['application/x-www-form-urlencoded']['schema']['required'] = required

    # Convert responses
    if 'responses' in operation:
        new_responses = {}
        for code, response in operation['responses'].items():
            if isinstance(response, dict) and '$ref' not in response:
                new_responses[code] = convert_response(response)
            else:
                new_responses[code] = response
        new_op['responses'] = new_responses

    # Remove v2-only fields
    # consumes, produces, schemes are removed (moved to operation level content types or servers)

    return new_op


def convert_spec(spec: Dict[str, Any]) -> Dict[str, Any]:
    """Convert Swagger 2.0 spec to OpenAPI 3.0.0"""
    new_spec = {}

    # Convert swagger version
    if spec.get('swagger') == '2.0':
        new_spec['openapi'] = '3.0.0'
    else:
        # Already v3 or not a swagger spec
        return spec

    # Copy basic fields
    for field in ['info', 'externalDocs', 'tags']:
        if field in spec:
            new_spec[field] = spec[field]

    # Convert servers
    host = spec.get('host', '')
    base_path = spec.get('basePath', '')
    schemes = spec.get('schemes', [])

    if host or base_path:
        servers = convert_server(host, base_path, schemes)
        if servers:
            new_spec['servers'] = servers

    # Convert paths
    if 'paths' in spec:
        new_paths = {}
        for path, path_item in spec['paths'].items():
            if not isinstance(path_item, dict):
                new_paths[path] = path_item
                continue

            new_path_item = {}

            # Handle $ref at path level
            if '$ref' in path_item:
                new_paths[path] = path_item
                continue

            # Convert parameters at path level
            if 'parameters' in path_item:
                new_params = []
                for param in path_item['parameters']:
                    if isinstance(param, dict):
                        converted = convert_parameter(param)
                        if converted:
                            new_params.append(converted)
                    else:
                        new_params.append(param)
                if new_params:
                    new_path_item['parameters'] = new_params

            # Convert operations
            for method in ['get', 'put', 'post', 'delete', 'options', 'head', 'patch']:
                if method in path_item:
                    new_path_item[method] = convert_operation(path_item[method])

            # Copy vendor extensions
            for key, value in path_item.items():
                if key.startswith('x-'):
                    new_path_item[key] = value

            new_paths[path] = new_path_item

        new_spec['paths'] = new_paths

    # Convert components
    components = {}

    if 'definitions' in spec:
        components['schemas'] = spec['definitions']

    if 'parameters' in spec:
        new_params = {}
        for name, param in spec['parameters'].items():
            if isinstance(param, dict):
                converted = convert_parameter(param)
                if converted:
                    new_params[name] = converted
            else:
                new_params[name] = param
        if new_params:
            components['parameters'] = new_params

    if 'responses' in spec:
        new_responses = {}
        for name, response in spec['responses'].items():
            if isinstance(response, dict):
                new_responses[name] = convert_response(response)
            else:
                new_responses[name] = response
        if new_responses:
            components['responses'] = new_responses

    if 'securityDefinitions' in spec:
        components['securitySchemes'] = spec['securityDefinitions']

    if components:
        new_spec['components'] = components

    # Copy security
    if 'security' in spec:
        new_spec['security'] = spec['security']

    # Copy vendor extensions
    for key, value in spec.items():
        if key.startswith('x-'):
            new_spec[key] = value

    # Copy id if present
    if 'id' in spec:
        new_spec['id'] = spec['id']

    return new_spec


def update_references(obj: Any) -> Any:
    """Recursively update $ref paths from v2 to v3"""
    if isinstance(obj, dict):
        new_obj = {}
        for key, value in obj.items():
            if key == '$ref' and isinstance(value, str):
                # Update reference paths
                value = value.replace('#/definitions/', '#/components/schemas/')
                value = value.replace('#/parameters/', '#/components/parameters/')
                value = value.replace('#/responses/', '#/components/responses/')
                value = value.replace('#/securityDefinitions/', '#/components/securitySchemes/')
                new_obj[key] = value
            else:
                new_obj[key] = update_references(value)
        return new_obj
    elif isinstance(obj, list):
        return [update_references(item) for item in obj]
    else:
        return obj


def convert_file(filepath: Path, dry_run: bool = False) -> bool:
    """Convert a single fixture file from v2 to v3"""
    try:
        with open(filepath, 'r', encoding='utf-8') as f:
            spec = json.load(f)

        # Check if it's a v2 spec
        if spec.get('swagger') != '2.0':
            print(f"Skipping {filepath} - not a Swagger 2.0 file")
            return False

        print(f"Converting {filepath}...")

        # Convert the spec
        new_spec = convert_spec(spec)

        # Update all $ref paths
        new_spec = update_references(new_spec)

        if not dry_run:
            # Write back to file
            with open(filepath, 'w', encoding='utf-8') as f:
                json.dump(new_spec, f, indent=2)
                f.write('\n')  # Add trailing newline

        return True

    except Exception as e:
        print(f"Error converting {filepath}: {e}")
        return False


def main():
    """Main conversion function"""
    import argparse

    parser = argparse.ArgumentParser(description='Convert Swagger 2.0 fixtures to OpenAPI 3.0.0')
    parser.add_argument('--dry-run', action='store_true', help='Show what would be converted without actually converting')
    parser.add_argument('--dir', default='fixtures', help='Directory to search for fixtures (default: fixtures)')

    args = parser.parse_args()

    fixtures_dir = Path(args.dir)
    if not fixtures_dir.exists():
        print(f"Error: Directory {fixtures_dir} does not exist")
        sys.exit(1)

    # Find all JSON files
    json_files = list(fixtures_dir.rglob('*.json'))
    print(f"Found {len(json_files)} JSON files")

    converted = 0
    skipped = 0

    for filepath in json_files:
        if convert_file(filepath, dry_run=args.dry_run):
            converted += 1
        else:
            skipped += 1

    print(f"\nConversion complete!")
    print(f"Converted: {converted}")
    print(f"Skipped: {skipped}")

    if args.dry_run:
        print("\nThis was a dry run. Use without --dry-run to actually convert files.")


if __name__ == '__main__':
    main()
