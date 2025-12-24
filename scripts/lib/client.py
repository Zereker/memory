"""Memory System API Client"""

import json
import urllib.request
import urllib.error
from dataclasses import dataclass, field
from typing import Optional


@dataclass
class Config:
    """API Configuration"""
    base_url: str = "http://localhost:8080"
    default_agent_id: str = "贾维斯"
    default_user_id: str = "阿信"
    timeout: int = 120

    @property
    def api_url(self) -> str:
        return f"{self.base_url}/api/v1"


@dataclass
class AddResult:
    """Add operation result"""
    success: bool
    episodes: int = 0
    entities: int = 0
    edges: int = 0
    summaries: int = 0
    error: str = ""

    @property
    def memory_count(self) -> int:
        return self.entities + self.edges


@dataclass
class RetrieveResult:
    """Retrieve operation result"""
    success: bool
    episodes: list = field(default_factory=list)
    entities: list = field(default_factory=list)
    edges: list = field(default_factory=list)
    summaries: list = field(default_factory=list)
    memory_context: str = ""
    error: str = ""

    @property
    def total(self) -> int:
        return len(self.episodes) + len(self.edges) + len(self.summaries)

    @property
    def all_memories(self) -> list:
        """Get all memories as unified list"""
        memories = []
        for ep in self.episodes:
            memories.append({
                "type": "episode",
                "content": ep.get("content", ""),
                "score": ep.get("score", 0),
            })
        for edge in self.edges:
            memories.append({
                "type": "edge",
                "content": edge.get("fact", ""),
                "score": edge.get("score", 0),
            })
        for summary in self.summaries:
            memories.append({
                "type": "summary",
                "content": summary.get("content", ""),
                "score": summary.get("score", 0),
            })
        return memories


class MemoryClient:
    """Memory System API Client"""

    def __init__(self, config: Optional[Config] = None):
        self.config = config or Config()

    def _http_post(self, url: str, data: dict, timeout: int = None) -> dict:
        """Send HTTP POST request"""
        timeout = timeout or self.config.timeout
        try:
            req = urllib.request.Request(
                url,
                data=json.dumps(data).encode("utf-8"),
                headers={"Content-Type": "application/json"},
                method="POST"
            )
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8") if e.fp else ""
            return {"success": False, "error": f"HTTP {e.code}: {e.reason} - {body}"}
        except urllib.error.URLError as e:
            return {"success": False, "error": str(e.reason)}
        except Exception as e:
            return {"success": False, "error": str(e)}

    def _http_get(self, url: str, timeout: int = 5) -> dict:
        """Send HTTP GET request"""
        try:
            req = urllib.request.Request(url, method="GET")
            with urllib.request.urlopen(req, timeout=timeout) as resp:
                return json.loads(resp.read().decode("utf-8"))
        except Exception as e:
            return {"success": False, "error": str(e)}

    def health_check(self) -> bool:
        """Check if server is healthy"""
        result = self._http_get(f"{self.config.base_url}/health")
        return result.get("success", False)

    def add(self, messages: list, session_id: str,
            agent_id: str = None, user_id: str = None) -> AddResult:
        """Add memories from conversation"""
        # Infer agent/user from messages if not provided
        _agent_id = agent_id or self.config.default_agent_id
        _user_id = user_id or self.config.default_user_id

        for msg in messages:
            if msg.get("role") == "user" and msg.get("name"):
                _user_id = msg["name"]
            elif msg.get("role") == "assistant" and msg.get("name"):
                _agent_id = msg["name"]

        payload = {
            "agent_id": _agent_id,
            "user_id": _user_id,
            "session_id": session_id,
            "messages": messages,
        }

        result = self._http_post(f"{self.config.api_url}/memories/add", payload)

        if result.get("success"):
            data = result.get("data", {})
            return AddResult(
                success=data.get("success", True),
                episodes=len(data.get("episodes", [])),
                entities=len(data.get("entities", [])),
                edges=len(data.get("edges", [])),
                summaries=len(data.get("summaries", [])),
            )
        return AddResult(success=False, error=result.get("error", "Unknown error"))

    def retrieve(self, query: str, agent_id: str = None, user_id: str = None,
                 session_id: str = "", limit: int = 10,
                 max_hops: int = 2, max_tokens: int = 0,
                 max_summaries: int = 0, max_edges: int = 0,
                 max_entities: int = 0, max_episodes: int = 0) -> RetrieveResult:
        """Retrieve memories by query

        Args:
            max_* params: -1 disables, 0 uses default, >0 custom value
        """
        payload = {
            "agent_id": agent_id or self.config.default_agent_id,
            "user_id": user_id or self.config.default_user_id,
            "session_id": session_id,
            "query": query,
            "limit": limit,
            "options": {
                "max_hops": max_hops,
                "max_tokens": max_tokens,
                "max_summaries": max_summaries,
                "max_edges": max_edges,
                "max_entities": max_entities,
                "max_episodes": max_episodes,
            }
        }

        result = self._http_post(f"{self.config.api_url}/memories/retrieve", payload, timeout=60)

        if result.get("success"):
            data = result.get("data", {})
            return RetrieveResult(
                success=data.get("success", True),
                episodes=data.get("episodes", []),
                entities=data.get("entities", []),
                edges=data.get("edges", []),
                summaries=data.get("summaries", []),
                memory_context=data.get("memory_context", ""),
            )
        return RetrieveResult(success=False, error=result.get("error", "Unknown error"))


def check_keywords(memories: list, keywords: list) -> tuple:
    """Check which keywords are found in memories"""
    all_content = " ".join([m.get("content", "") for m in memories])
    matched = []
    missing = []
    for kw in keywords:
        if kw.lower() in all_content.lower():
            matched.append(kw)
        else:
            missing.append(kw)
    return matched, missing
