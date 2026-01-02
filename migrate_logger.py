#!/usr/bin/env python3
"""Script to migrate logger.Fields to sdklog field helpers in ingester_loader.go"""

import re
import sys

def migrate_logger_fields(content):
    """Replace logger.Fields with sdklog field helpers"""
    
    # Pattern to match logger.Fields{...} blocks
    pattern = r'logger\.Fields\{\s*Component:\s*"([^"]+)",\s*Operation:\s*"([^"]+)"(.*?)\}'
    
    def replace_fields(match):
        component = match.group(1)
        operation = match.group(2)
        rest = match.group(3)
        
        # Extract additional fields
        fields = []
        
        # Extract Namespace
        namespace_match = re.search(r'Namespace:\s*([^,\n]+)', rest)
        if namespace_match:
            ns_value = namespace_match.group(1).strip()
            fields.append(f'sdklog.String("namespace", {ns_value})')
        
        # Extract Source
        source_match = re.search(r'Source:\s*([^,\n]+)', rest)
        if source_match:
            src_value = source_match.group(1).strip()
            fields.append(f'sdklog.String("source", {src_value})')
        
        # Extract Error
        error_match = re.search(r'Error:\s*([^,\n]+)', rest)
        if error_match:
            err_value = error_match.group(1).strip()
            fields.append(f'sdklog.Error({err_value})')
        
        # Extract Additional map
        additional_match = re.search(r'Additional:\s*map\[string\]interface\{\}(.*?)(?=\s*\)|\s*Source:|\s*Error:|\s*Namespace:)', rest, re.DOTALL)
        if additional_match:
            additional_content = additional_match.group(1)
            # Extract key-value pairs from the map
            kv_pairs = re.findall(r'"([^"]+)":\s*([^,\n\}]+)', additional_content)
            for key, value in kv_pairs:
                value = value.strip()
                # Try to determine type
                if value.startswith('"') or value.startswith("'"):
                    fields.append(f'sdklog.String("{key}", {value})')
                elif re.match(r'^\d+$', value):
                    fields.append(f'sdklog.Int("{key}", {value})')
                elif re.match(r'^\d+\.\d+', value):
                    fields.append(f'sdklog.Float64("{key}", {value})')
                else:
                    fields.append(f'sdklog.String("{key}", {value})')
        
        # Build the replacement
        all_fields = [f'sdklog.Operation("{operation}")'] + fields
        return ', '.join(all_fields)
    
    # Replace logger.Info/Warn/Error/Debug calls
    # First, replace logger.Info/Warn/Error/Debug calls
    lines = content.split('\n')
    new_lines = []
    i = 0
    logger_declared = set()  # Track functions where logger is declared
    
    while i < len(lines):
        line = lines[i]
        
        # Check if this is a function start (to declare logger)
        if re.match(r'^\s*func\s+', line):
            # Extract function name to track
            func_match = re.search(r'func\s+\([^)]+\)\s+(\w+)', line)
            if func_match:
                func_name = func_match.group(1)
                logger_declared.add(func_name)
        
        # Replace logger.Fields{...} blocks
        if 'logger.Fields{' in line:
            # Multi-line replacement - find the complete block
            block_start = i
            block_lines = [line]
            brace_count = line.count('{') - line.count('}')
            i += 1
            
            while i < len(lines) and brace_count > 0:
                block_lines.append(lines[i])
                brace_count += lines[i].count('{') - lines[i].count('}')
                i += 1
            
            block = '\n'.join(block_lines)
            # This is complex - let's use a simpler approach
            # Just mark for manual replacement
            new_lines.append(f"// TODO: Replace logger.Fields block\n{block}")
            continue
        
        # Replace logger.Info/Warn/Error/Debug with logger := sdklog.NewLogger if not declared
        if re.match(r'^\s+logger\.(Info|Warn|Error|Debug)\(', line):
            # Check if logger is declared in this function
            # For now, just add declaration before first logger call
            if not any('logger := sdklog.NewLogger' in prev_line for prev_line in new_lines[-10:]):
                new_lines.append('\tlogger := sdklog.NewLogger("zen-watcher-config")')
        
        new_lines.append(line)
        i += 1
    
    return '\n'.join(new_lines)

if __name__ == '__main__':
    print("This script is a helper - doing manual replacement instead", file=sys.stderr)
    sys.exit(1)

