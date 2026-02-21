
import sys

def fix_file(filename):
    with open(filename, 'r', encoding='utf-8') as f:
        lines = f.readlines()

    # Targets (1-indexed line numbers translated to 0-indexed)
    targets = [
        (1300, 1301), # StateTo
        (1340, 1341), # StateTariff
        (1420, 1421) # StateDateTime (Time selection)
    ]

    # We iterate backwards to avoid line number shifting issues
    for start, end in sorted(targets, reverse=True):
        # Line numbers are 1-based
        idx_start = start - 1
        idx_end = end - 1
        
        # Verify content briefly (check for .Inline in both)
        if '.Inline(rows...)' in lines[idx_start] and '.Inline(menu.Row(menu.Data(' in lines[idx_end]:
            print(f"Fixing lines {start}-{end}...")
            # Detect indentation from the first line
            indent = lines[idx_start][:lines[idx_start].find('menu.Inline')]
            
            new_lines = [
                f'{indent}rows = append(rows, menu.Row(menu.Data("❌ Отменить", "cl_cancel")))\n',
                f'{indent}menu.Inline(rows...)\n'
            ]
            lines[idx_start:idx_end+1] = new_lines
        else:
            print(f"Warning: Unexpected content at lines {start}-{end}. Skipping.")
            print(f"Line {start}: {repr(lines[idx_start])}")
            print(f"Line {end}: {repr(lines[idx_end])}")

    with open(filename, 'w', encoding='utf-8') as f:
        f.writelines(lines)

if __name__ == "__main__":
    if len(sys.argv) > 1:
        fix_file(sys.argv[1])
    else:
        print("Usage: python fix_kb.py <filename>")
