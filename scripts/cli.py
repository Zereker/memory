#!/usr/bin/env python3
"""
Memory System CLI

Usage:
    python3 scripts/cli.py <command> [options]

Commands:
    status              Show service status
    init [--index X]    Initialize indexes
    clear [--index X]   Clear all data
    reset [--index X]   Clear + init
    test [MODE]         Run tests (quick|store|retrieve|full|all)
    preview [MODE]      Full workflow: reset + test
"""

import sys
import os
import time
import json
import argparse
from pathlib import Path
from datetime import datetime

# Add scripts directory to path
SCRIPT_DIR = Path(__file__).parent
sys.path.insert(0, str(SCRIPT_DIR))

from lib.client import MemoryClient, Config, check_keywords
from lib.infra import InfraManager

# ============================================================
# Constants
# ============================================================

DATA_FILE = Path("data/test_data_complete.json")

# Colors
GREEN = "\033[92m"
RED = "\033[91m"
YELLOW = "\033[93m"
RESET = "\033[0m"


def get_report_files():
    """Get report file paths with timestamp"""
    ts = datetime.now().strftime("%Y-%m-%d_%H%M%S")
    return (
        Path(f"data/test_results_{ts}.md"),
        Path(f"data/test_results_detail_{ts}.md"),
    )


def log_info(msg): print(f"{GREEN}[INFO]{RESET} {msg}")
def log_error(msg): print(f"{RED}[ERROR]{RESET} {msg}")
def log_warn(msg): print(f"{YELLOW}[WARN]{RESET} {msg}")


def load_test_data() -> dict:
    """Load test data"""
    with open(DATA_FILE, "r", encoding="utf-8") as f:
        return json.load(f)


def print_result(name: str, success: bool):
    """Print operation result"""
    status = f"{GREEN}OK{RESET}" if success else f"{RED}FAILED{RESET}"
    print(f"  {name}... {status}")


# ============================================================
# Commands
# ============================================================

def cmd_status(args):
    """Show service status"""
    infra = InfraManager()
    client = MemoryClient()

    print("Service Status")
    print("-" * 40)

    # Memory server
    server_ok = client.health_check()
    status = f"{GREEN}OK{RESET}" if server_ok else f"{RED}OFFLINE{RESET}"
    print(f"Memory Server:  {status}")

    # OpenSearch
    os_status = infra.opensearch_status()
    if os_status["online"]:
        count = infra.opensearch_count(args.index)
        print(f"OpenSearch:     {GREEN}OK{RESET} (v{os_status['version']}, {count} docs)")
    else:
        print(f"OpenSearch:     {RED}OFFLINE{RESET}")

    # Neo4j
    neo_status = infra.neo4j_status()
    if neo_status["online"]:
        count = infra.neo4j_count()
        print(f"Neo4j:          {GREEN}OK{RESET} ({count} nodes)")
    else:
        print(f"Neo4j:          {RED}OFFLINE{RESET}")


def get_index(args) -> str:
    """Get index name from args"""
    return args.index


def cmd_init(args):
    """Initialize indexes"""
    infra = InfraManager()
    index = get_index(args)

    print(f"Initializing index '{index}'...")
    print_result("OpenSearch", infra.opensearch_create(index))
    print_result("Neo4j", infra.neo4j_init())
    print("Done.")


def cmd_clear(args):
    """Clear all data"""
    infra = InfraManager()
    index = get_index(args)

    print(f"Clearing index '{index}'...")
    print_result("OpenSearch", infra.opensearch_delete(index))
    print_result("Neo4j", infra.neo4j_clear())
    print("Done.")


def cmd_reset(args):
    """Reset: clear + init"""
    cmd_clear(args)
    print()
    cmd_init(args)


def cmd_test(args):
    """Run tests"""
    mode = args.mode if hasattr(args, 'mode') and args.mode else "all"
    runner = TestRunner()

    # Health check
    if not runner.check_server():
        sys.exit(1)
    print()

    success = True
    if mode == "quick":
        success = runner.run_quick()
    elif mode == "store":
        success = runner.run_store()
    elif mode == "retrieve":
        success = runner.run_retrieve()
        runner.print_summary()
    elif mode == "full":
        success = runner.run_full()
        runner.print_summary()
    elif mode == "all":
        runner.run_quick()
        print()
        success = runner.run_full()
        runner.print_summary()

    # Generate report
    if mode in ["full", "all", "retrieve"]:
        runner.generate_report()

    return success


def cmd_preview(args):
    """Full workflow: reset + test"""
    mode = args.mode if hasattr(args, 'mode') and args.mode else "full"

    # Reset
    cmd_reset(args)
    print()

    # Wait for services
    print("Waiting for services (1s)...")
    time.sleep(1)
    print()

    # Test
    args.mode = mode
    return cmd_test(args)


# ============================================================
# Test Runner
# ============================================================

class TestRunner:
    """Test runner"""

    def __init__(self):
        self.client = MemoryClient()
        self.results = {
            "test_date": datetime.now().strftime("%Y-%m-%d %H:%M:%S"),
            "store_results": [],
            "recall_results": {"basic_recall": [], "temporal": [], "causal": []},
        }

    def check_server(self) -> bool:
        print("Checking server...", end=" ", flush=True)
        if not self.client.health_check():
            print(f"{RED}FAILED{RESET}")
            log_error("Server not running. Start with: ./bin/memory -config configs/config.toml")
            return False
        print(f"{GREEN}OK{RESET}")
        return True

    def run_quick(self) -> bool:
        """Quick smoke test"""
        print("=" * 50)
        print("Quick Test")
        print("=" * 50)

        tests = [
            ("Weather", [
                {"role": "user", "name": "小明", "content": "今天天气真好"},
                {"role": "assistant", "content": "是啊，适合出去走走！"}
            ]),
            ("Programming", [
                {"role": "user", "name": "小明", "content": "我在学 Python"},
                {"role": "assistant", "content": "Python 很适合入门！"}
            ]),
        ]

        for i, (name, messages) in enumerate(tests, 1):
            print(f"[{i}/{len(tests)}] {name}...", end=" ", flush=True)
            r = self.client.add(messages, f"quick_{i}", "test", "小明")
            if r.success:
                print(f"{GREEN}OK{RESET} (ent:{r.entities}, edge:{r.edges})")
            else:
                print(f"{RED}FAILED{RESET}: {r.error}")
                return False
            time.sleep(0.5)

        print("\nQuick test passed!")
        return True

    def run_store(self) -> bool:
        """Store test data"""
        print("=" * 50)
        print("Store Test")
        print("=" * 50)

        data = load_test_data()
        convs = data["conversations"]
        success = 0

        for i, conv in enumerate(convs, 1):
            print(f"[{i:2}/{len(convs)}] {conv['session_date']}...", end=" ", flush=True)
            r = self.client.add(conv["messages"], conv["session_id"])

            if r.success:
                success += 1
                print(f"{GREEN}OK{RESET} (ent:{r.entities}, edge:{r.edges})")
            else:
                print(f"{RED}FAIL{RESET}: {r.error[:40]}")

            self.results["store_results"].append({
                "index": i, "session_date": conv["session_date"],
                "success": r.success, "entities": r.entities, "edges": r.edges,
            })
            time.sleep(0.3)

        print(f"\nStore: {success}/{len(convs)}")
        return success == len(convs)

    def run_retrieve(self) -> bool:
        """Retrieval test"""
        print("=" * 50)
        print("Retrieve Test")
        print("=" * 50)

        data = load_test_data()
        questions = data["test_questions"]
        total_passed = 0
        total_tests = 0

        for cat, name in [("basic_recall", "Basic"), ("temporal", "Temporal"), ("causal", "Causal")]:
            tests = questions.get(cat, [])
            if not tests:
                continue

            print(f"\n[{name}]")
            for i, t in enumerate(tests, 1):
                q = t["message"][:35] + "..." if len(t["message"]) > 35 else t["message"]
                print(f"  {i}. {q}", end=" ", flush=True)

                r = self.client.retrieve(t["message"])
                memories = r.all_memories
                matched, missing = check_keywords(memories, t["keywords"])
                rate = len(matched) / len(t["keywords"]) * 100 if t["keywords"] else 0
                passed = rate >= 50

                ep, ed, su = len(r.episodes), len(r.edges), len(r.summaries)
                status = f"{GREEN}PASS{RESET}" if passed else f"{RED}FAIL{RESET}"
                print(f"{status} ({len(matched)}/{len(t['keywords'])}kw, Ep:{ep} Ed:{ed} Su:{su})")

                self.results["recall_results"][cat].append({
                    "index": i, "query": t["message"], "keywords": t["keywords"],
                    "matched_keywords": matched, "missing_keywords": missing,
                    "match_rate": rate, "passed": passed,
                    "episode_count": ep, "edge_count": ed, "summary_count": su,
                    "memories_detail": memories,
                })

                total_tests += 1
                if passed:
                    total_passed += 1
                time.sleep(0.3)

        rate = total_passed / total_tests * 100 if total_tests else 0
        print(f"\nRetrieve: {total_passed}/{total_tests} ({rate:.0f}%)")
        return total_passed == total_tests

    def run_full(self) -> bool:
        """Full test"""
        store_ok = self.run_store()
        print("\nWaiting for index (2s)...")
        time.sleep(2)
        print()
        retrieve_ok = self.run_retrieve()
        return store_ok and retrieve_ok

    def print_summary(self):
        """Print summary"""
        print()
        print("=" * 50)
        print("Summary")
        print("=" * 50)

        for cat in ["basic_recall", "temporal", "causal"]:
            tests = self.results["recall_results"].get(cat, [])
            if tests:
                passed = sum(1 for t in tests if t["passed"])
                print(f"  {cat}: {passed}/{len(tests)}")

        total = sum(len(v) for v in self.results["recall_results"].values())
        passed = sum(1 for cat in self.results["recall_results"].values() for t in cat if t["passed"])
        if total:
            print(f"  Overall: {passed}/{total} ({passed/total*100:.0f}%)")

    def generate_report(self):
        """Generate reports"""
        report_file, detail_file = get_report_files()

        # Summary
        lines = ["# Test Results\n", f"Date: {self.results['test_date']}\n"]

        for cat, name in [("basic_recall", "Basic"), ("temporal", "Temporal"), ("causal", "Causal")]:
            tests = self.results["recall_results"].get(cat, [])
            if tests:
                lines.append(f"\n## {name}\n")
                lines.append("| # | Query | Ep | Ed | Su | KW | Status |")
                lines.append("|---|-------|---|---|---|---|--------|")
                for r in tests:
                    q = r["query"][:25] + "..." if len(r["query"]) > 25 else r["query"]
                    kw = f"{len(r['matched_keywords'])}/{len(r['keywords'])}"
                    status = "PASS" if r["passed"] else "FAIL"
                    lines.append(f"| {r['index']} | {q} | {r['episode_count']} | {r['edge_count']} | {r['summary_count']} | {kw} | {status} |")

        with open(report_file, "w") as f:
            f.write("\n".join(lines))

        # Detail
        lines = ["# Detailed Results\n", f"Date: {self.results['test_date']}\n"]
        for cat, name in [("basic_recall", "Basic"), ("temporal", "Temporal"), ("causal", "Causal")]:
            for r in self.results["recall_results"].get(cat, []):
                status = "PASS" if r["passed"] else "FAIL"
                lines.append(f"\n## [{status}] {r['query']}\n")
                lines.append(f"- Matched: {', '.join(r['matched_keywords']) or 'None'}")
                lines.append(f"- Missing: {', '.join(r['missing_keywords']) or 'None'}")
                if r.get("memories_detail"):
                    lines.append("\n**Memories:**")
                    for m in r["memories_detail"]:
                        lines.append(f"- [{m['type']}] {m['content']}")

        with open(detail_file, "w") as f:
            f.write("\n".join(lines))

        print(f"\nReports: {report_file}, {detail_file}")


# ============================================================
# Main
# ============================================================

COMMANDS = {
    "status": cmd_status,
    "init": cmd_init,
    "clear": cmd_clear,
    "reset": cmd_reset,
    "test": cmd_test,
    "preview": cmd_preview,
}


def main():
    parser = argparse.ArgumentParser(
        description="Memory System CLI",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Commands:
  status              Show service status
  init [--index X]    Initialize indexes (default: memories)
  clear [--index X]   Clear index data (default: memories)
  reset [--index X]   Clear + init (default: memories)
  test [MODE]         Run tests (quick|store|retrieve|full|all)
  preview [MODE]      Reset + test

Examples:
  %(prog)s status
  %(prog)s init
  %(prog)s init --index memories_test
  %(prog)s clear --index memories_test
  %(prog)s test quick
"""
    )

    parser.add_argument("command", choices=list(COMMANDS.keys()))
    parser.add_argument("mode", nargs="?", default="all", help="Test mode for test/preview")
    parser.add_argument("--index", "-i", default="memories", help="Index name")

    args = parser.parse_args()

    # Change to project root
    project_root = SCRIPT_DIR.parent
    os.chdir(project_root)

    success = COMMANDS[args.command](args)
    sys.exit(0 if success is None or success else 1)


if __name__ == "__main__":
    main()
