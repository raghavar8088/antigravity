#!/usr/bin/env python3
"""
Autonomous AI Bot for Code Analysis, Issue Detection, and Automated Fixes
Uses Claude API to analyze Python code and automatically apply improvements
"""

import os
import json
import sys
from pathlib import Path
from datetime import datetime
from typing import Optional

try:
    import anthropic
except ImportError:
    anthropic = None

if hasattr(sys.stdout, "reconfigure"):
    sys.stdout.reconfigure(errors="ignore")

client = None


def get_claude_client():
    """Initialize Claude client lazily so startup can fail gracefully."""
    global client
    if client is not None:
        return client
    if anthropic is None:
        raise RuntimeError("anthropic package not installed. Run: pip install -r requirements.txt")
    client = anthropic.Anthropic(api_key=os.environ.get("ANTHROPIC_API_KEY"))
    return client

class CodeAnalyzer:
    """Analyzes Python code using Claude API"""

    def __init__(self, app_root: str):
        self.app_root = Path(app_root)
        self.python_files = []
        self.analysis_results = {}
        self.upgrade_plans = {}
        self.applied_fixes = {}

    def scan_application(self) -> list:
        """Scan and collect all Python files from the application"""
        print(f"\n🔍 Scanning application at: {self.app_root}")

        # Exclude venv, node_modules, __pycache__
        exclude_dirs = {'.venv', 'venv', 'node_modules', '__pycache__', '.git', '.pytest_cache'}

        for py_file in self.app_root.rglob('*.py'):
            # Skip venv and node_modules
            if not any(excluded in py_file.parts for excluded in exclude_dirs):
                self.python_files.append(py_file)

        print(f"✅ Found {len(self.python_files)} Python files to analyze")
        return self.python_files

    def read_file_content(self, file_path: Path) -> str:
        """Read file content safely"""
        try:
            return file_path.read_text(encoding='utf-8')
        except Exception as e:
            print(f"⚠️  Could not read {file_path}: {e}")
            return ""

    def analyze_file_with_claude(self, file_path: Path, content: str) -> dict:
        """Use Claude to analyze a single file"""
        relative_path = file_path.relative_to(self.app_root)

        prompt = f"""Analyze this Python file for issues and improvements:

File: {relative_path}

Code:
```python
{content}
```

Provide analysis in this JSON format:
{{
    "file": "{relative_path}",
    "issues": [
        {{
            "severity": "high|medium|low",
            "category": "code_quality|performance|security|dependencies|best_practices",
            "issue": "description",
            "line_number": number or null,
            "fix_suggestion": "how to fix it"
        }}
    ],
    "code_quality_score": 0-100,
    "summary": "brief summary of findings"
}}

Focus on:
1. Code quality & best practices
2. Performance optimization
3. Security issues
4. Dependency problems
5. Bug-prone patterns
"""

        try:
            message = get_claude_client().messages.create(
                model="claude-opus-4-6",
                max_tokens=2048,
                messages=[
                    {"role": "user", "content": prompt}
                ]
            )

            # Extract JSON from response
            response_text = message.content[0].text

            # Find JSON in response
            json_start = response_text.find('{')
            json_end = response_text.rfind('}') + 1

            if json_start != -1 and json_end > json_start:
                json_str = response_text[json_start:json_end]
                analysis = json.loads(json_str)
            else:
                analysis = {
                    "file": str(relative_path),
                    "issues": [],
                    "code_quality_score": 0,
                    "summary": "Could not parse analysis"
                }

            return analysis

        except Exception as e:
            print(f"❌ Error analyzing {file_path}: {e}")
            return {
                "file": str(relative_path),
                "issues": [],
                "code_quality_score": 0,
                "summary": f"Error: {str(e)}"
            }

    def analyze_all_files(self) -> dict:
        """Analyze all Python files in the application"""
        print("\n🤖 Analyzing files with Claude AI...")

        for i, file_path in enumerate(self.python_files, 1):
            print(f"  [{i}/{len(self.python_files)}] Analyzing {file_path.relative_to(self.app_root)}...")

            content = self.read_file_content(file_path)
            if content:
                analysis = self.analyze_file_with_claude(file_path, content)
                self.analysis_results[str(file_path)] = analysis

        return self.analysis_results

    def generate_upgrade_plan(self) -> dict:
        """Generate comprehensive upgrade plan based on analysis"""
        print("\n📋 Generating upgrade plan...")

        all_issues = []
        total_files = len(self.analysis_results)
        avg_quality = 0

        for file_path, analysis in self.analysis_results.items():
            if analysis.get("issues"):
                all_issues.extend([
                    {**issue, "file": analysis.get("file")}
                    for issue in analysis["issues"]
                ])
            avg_quality += analysis.get("code_quality_score", 0)

        avg_quality = avg_quality // max(total_files, 1)

        # Group issues by severity and category
        high_priority = [i for i in all_issues if i.get("severity") == "high"]
        medium_priority = [i for i in all_issues if i.get("severity") == "medium"]
        low_priority = [i for i in all_issues if i.get("severity") == "low"]

        upgrade_plan = {
            "timestamp": datetime.now().isoformat(),
            "total_files_analyzed": total_files,
            "total_issues_found": len(all_issues),
            "avg_code_quality_score": avg_quality,
            "priority_breakdown": {
                "high": len(high_priority),
                "medium": len(medium_priority),
                "low": len(low_priority)
            },
            "high_priority_issues": high_priority[:10],
            "recommended_actions": [
                {
                    "priority": "1",
                    "action": "Fix high-severity security and performance issues",
                    "estimated_effort": "High",
                    "issues_affected": len(high_priority)
                },
                {
                    "priority": "2",
                    "action": "Refactor for code quality and best practices",
                    "estimated_effort": "Medium",
                    "issues_affected": len(medium_priority)
                },
                {
                    "priority": "3",
                    "action": "Apply optimizations and minor improvements",
                    "estimated_effort": "Low",
                    "issues_affected": len(low_priority)
                }
            ]
        }

        self.upgrade_plans = upgrade_plan
        return upgrade_plan

    def apply_fixes_to_file(self, file_path: Path) -> bool:
        """Apply Claude-suggested fixes to a file"""
        try:
            content = self.read_file_content(file_path)
            if not content:
                return False

            file_analysis = self.analysis_results.get(str(file_path), {})
            issues = file_analysis.get("issues", [])

            if not issues:
                return True

            # Create fix request for Claude
            prompt = f"""Apply the following fixes to this Python code:

File: {file_path.name}

Current Code:
```python
{content}
```

Issues to fix:
{json.dumps(issues[:5], indent=2)}

Requirements:
1. Keep the same functionality
2. Apply fixes for each issue listed
3. Maintain code structure
4. Add comments for significant changes
5. Return ONLY the fixed code in a Python code block

Fixed Code:
```python
"""

            message = get_claude_client().messages.create(
                model="claude-opus-4-6",
                max_tokens=4096,
                messages=[
                    {"role": "user", "content": prompt}
                ]
            )

            response_text = message.content[0].text

            # Extract code from response
            code_start = response_text.find('```python\n') + len('```python\n')
            code_end = response_text.rfind('```')

            if code_start > len('```python\n') - 1 and code_end > code_start:
                fixed_code = response_text[code_start:code_end].strip()

                # Save fixed code
                backup_path = str(file_path) + '.backup'
                file_path.write_text(content, encoding='utf-8')
                Path(backup_path).write_text(content, encoding='utf-8')

                file_path.write_text(fixed_code, encoding='utf-8')

                self.applied_fixes[str(file_path)] = {
                    "status": "success",
                    "issues_fixed": len(issues[:5]),
                    "backup_created": backup_path
                }

                return True

            return False

        except Exception as e:
            print(f"❌ Error applying fixes to {file_path}: {e}")
            self.applied_fixes[str(file_path)] = {
                "status": "failed",
                "error": str(e)
            }
            return False

    def apply_all_fixes(self) -> dict:
        """Apply fixes to all files with high-priority issues"""
        print("\n🔧 Applying automated fixes...")

        # Sort files by issue severity
        files_to_fix = []
        for file_path, analysis in self.analysis_results.items():
            issues = analysis.get("issues", [])
            high_severity = any(i.get("severity") == "high" for i in issues)

            if high_severity or len(issues) > 3:
                files_to_fix.append((file_path, len(issues)))

        files_to_fix.sort(key=lambda x: x[1], reverse=True)

        for i, (file_path, issue_count) in enumerate(files_to_fix[:10], 1):
            print(f"  [{i}] Fixing {Path(file_path).relative_to(self.app_root)} ({issue_count} issues)...")
            self.apply_fixes_to_file(Path(file_path))

        return self.applied_fixes

    def generate_report(self, output_file: Optional[str] = None) -> str:
        """Generate comprehensive analysis and fix report"""
        report = []
        report.append("=" * 80)
        report.append("AUTONOMOUS AI BOT - CODE ANALYSIS AND UPGRADE REPORT")
        report.append("=" * 80)
        report.append(f"\nGenerated: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}")
        report.append(f"Application Root: {self.app_root}")
        report.append(f"Total Files Analyzed: {len(self.python_files)}")

        # Upgrade Plan Summary
        report.append("\n" + "=" * 80)
        report.append("UPGRADE PLAN SUMMARY")
        report.append("=" * 80)

        if self.upgrade_plans:
            plan = self.upgrade_plans
            report.append(f"\nCode Quality Score (Average): {plan.get('avg_code_quality_score', 0)}/100")
            report.append(f"Total Issues Found: {plan.get('total_issues_found', 0)}")

            priorities = plan.get('priority_breakdown', {})
            report.append(f"\nIssue Breakdown:")
            report.append(f"  🔴 High Priority: {priorities.get('high', 0)}")
            report.append(f"  🟠 Medium Priority: {priorities.get('medium', 0)}")
            report.append(f"  🟡 Low Priority: {priorities.get('low', 0)}")

            report.append(f"\nRecommended Actions:")
            for action in plan.get('recommended_actions', []):
                report.append(f"  {action['priority']}. {action['action']} (Effort: {action['estimated_effort']})")

        # Applied Fixes Summary
        report.append("\n" + "=" * 80)
        report.append("APPLIED FIXES SUMMARY")
        report.append("=" * 80)

        if self.applied_fixes:
            successful_fixes = sum(1 for f in self.applied_fixes.values() if f.get('status') == 'success')
            failed_fixes = sum(1 for f in self.applied_fixes.values() if f.get('status') == 'failed')

            report.append(f"\n✅ Successfully Fixed: {successful_fixes} files")
            report.append(f"❌ Failed: {failed_fixes} files")

            report.append(f"\nFixed Files:")
            for file_path, result in self.applied_fixes.items():
                rel_path = Path(file_path).relative_to(self.app_root)
                if result.get('status') == 'success':
                    report.append(f"  ✅ {rel_path}")
                    report.append(f"     Issues Fixed: {result.get('issues_fixed', 0)}")
                    report.append(f"     Backup: {result.get('backup_created', 'N/A')}")
        else:
            report.append("\nNo fixes applied yet.")

        # File-by-file Analysis
        report.append("\n" + "=" * 80)
        report.append("DETAILED ANALYSIS - BY FILE")
        report.append("=" * 80)

        for file_path, analysis in self.analysis_results.items():
            rel_path = Path(file_path).relative_to(self.app_root)
            report.append(f"\n📄 {rel_path}")
            report.append(f"   Quality Score: {analysis.get('code_quality_score', 0)}/100")
            report.append(f"   Issues Found: {len(analysis.get('issues', []))}")

            for issue in analysis.get('issues', [])[:5]:
                severity = issue.get('severity', 'unknown').upper()
                category = issue.get('category', 'general')
                report.append(f"   - [{severity}] {category}: {issue.get('issue', 'N/A')}")
                if issue.get('fix_suggestion'):
                    report.append(f"     Fix: {issue['fix_suggestion']}")

        report.append("\n" + "=" * 80)
        report.append("END OF REPORT")
        report.append("=" * 80)

        report_text = "\n".join(report)

        if output_file:
            Path(output_file).write_text(report_text)
            print(f"\n📄 Report saved to: {output_file}")

        return report_text


def main():
    """Main execution flow"""
    print("""
    ╔═══════════════════════════════════════════════════════════════╗
    ║     AUTONOMOUS AI BOT - CODE ANALYZER & AUTO-FIXER             ║
    ║     Powered by Claude AI                                      ║
    ╚═══════════════════════════════════════════════════════════════╝
    """)

    # Verify API key
    if not os.environ.get("ANTHROPIC_API_KEY"):
        print("❌ Error: ANTHROPIC_API_KEY environment variable not set")
        print("Please set your Claude API key:")
        print("  Windows: set ANTHROPIC_API_KEY=your_key")
        print("  Linux/Mac: export ANTHROPIC_API_KEY=your_key")
        return

    if anthropic is None:
        print("Error: anthropic package is not installed")
        print("Please run: pip install -r requirements.txt")
        return

    app_root = r"C:\Trading apllication"

    # Initialize analyzer
    analyzer = CodeAnalyzer(app_root)

    # Execute analysis pipeline
    print("\n📍 Starting automated analysis pipeline...")

    # Step 1: Scan application
    analyzer.scan_application()

    # Step 2: Analyze all files
    analyzer.analyze_all_files()

    # Step 3: Generate upgrade plan
    plan = analyzer.generate_upgrade_plan()
    print(f"\n✨ Upgrade Plan Generated:")
    print(f"   - Total Issues: {plan.get('total_issues_found')}")
    print(f"   - High Priority: {plan.get('priority_breakdown', {}).get('high', 0)}")
    print(f"   - Avg Code Quality: {plan.get('avg_code_quality_score', 0)}/100")

    # Step 4: Apply fixes (fully autonomous)
    analyzer.apply_all_fixes()

    # Step 5: Generate report
    report_path = r"C:\Trading apllication\AI_BOT_ANALYSIS_REPORT.txt"
    report = analyzer.generate_report(report_path)

    # Print summary to console
    print("\n" + "=" * 80)
    print("🎉 ANALYSIS COMPLETE!")
    print("=" * 80)
    print(f"Total Files: {len(analyzer.python_files)}")
    print(f"Issues Found: {plan.get('total_issues_found', 0)}")
    print(f"Files Fixed: {len([f for f in analyzer.applied_fixes.values() if f.get('status') == 'success'])}")
    print(f"Report: {report_path}")
    print("=" * 80)


if __name__ == "__main__":
    main()
