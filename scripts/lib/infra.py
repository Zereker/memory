"""Infrastructure management for OpenSearch and Neo4j"""

import json
import urllib.request
import urllib.error
from dataclasses import dataclass


@dataclass
class InfraConfig:
    """Infrastructure connection configuration"""
    opensearch_url: str = "http://localhost:9200"
    embedding_dim: int = 4096
    neo4j_url: str = "http://localhost:7474"
    neo4j_user: str = "neo4j"
    neo4j_pass: str = "YOUR_NEO4J_PASSWORD"


class InfraManager:
    """Manage OpenSearch and Neo4j infrastructure"""

    def __init__(self, config: InfraConfig = None):
        self.config = config or InfraConfig()

    def _http_request(self, url: str, method: str = "GET",
                      data: dict = None, auth: tuple = None) -> dict:
        """Send HTTP request and return result"""
        try:
            req = urllib.request.Request(url, method=method)
            req.add_header("Content-Type", "application/json")

            if auth:
                import base64
                credentials = base64.b64encode(f"{auth[0]}:{auth[1]}".encode()).decode()
                req.add_header("Authorization", f"Basic {credentials}")

            body = json.dumps(data).encode() if data else None
            with urllib.request.urlopen(req, body, timeout=10) as resp:
                return {"success": True, "data": json.loads(resp.read().decode())}
        except urllib.error.HTTPError as e:
            return {"success": False, "error": f"HTTP {e.code}", "code": e.code}
        except Exception as e:
            return {"success": False, "error": str(e)}

    # ========== OpenSearch Operations ==========

    def opensearch_status(self) -> dict:
        """Check OpenSearch cluster status"""
        result = self._http_request(self.config.opensearch_url)
        if result["success"]:
            return {
                "online": True,
                "version": result["data"].get("version", {}).get("number", "unknown")
            }
        return {"online": False, "error": result.get("error")}

    def opensearch_count(self, index: str) -> int:
        """Get document count for specified index"""
        url = f"{self.config.opensearch_url}/{index}/_count"
        result = self._http_request(url)
        if result["success"]:
            return result["data"].get("count", 0)
        return 0

    def opensearch_exists(self, index: str) -> bool:
        """Check if index exists"""
        url = f"{self.config.opensearch_url}/{index}"
        result = self._http_request(url, method="HEAD")
        return result["success"]

    def opensearch_delete(self, index: str) -> bool:
        """Delete specified index"""
        url = f"{self.config.opensearch_url}/{index}"
        result = self._http_request(url, method="DELETE")
        return result["success"] or result.get("code") == 404

    def opensearch_create(self, index: str) -> bool:
        """Create index with k-NN mappings for memory storage"""
        url = f"{self.config.opensearch_url}/{index}"

        mapping = {
            "settings": {
                "index": {
                    "knn": True,
                    "knn.algo_param.ef_search": 100,
                    "number_of_shards": 1,
                    "number_of_replicas": 0
                }
            },
            "mappings": {
                "dynamic": True,
                "properties": {
                    # 向量字段
                    "embedding": {
                        "type": "knn_vector",
                        "dimension": self.config.embedding_dim,
                        "method": {
                            "name": "hnsw",
                            "space_type": "cosinesimil"
                        }
                    },
                    "topic_embedding": {
                        "type": "knn_vector",
                        "dimension": self.config.embedding_dim,
                        "method": {
                            "name": "hnsw",
                            "space_type": "cosinesimil"
                        }
                    },
                    # 通用字段
                    "id": {"type": "keyword"},
                    "type": {"type": "keyword"},  # episode, entity, edge, summary
                    "agent_id": {"type": "keyword"},
                    "user_id": {"type": "keyword"},
                    "session_id": {"type": "keyword"},
                    "status": {"type": "keyword"},
                    # Episode 字段
                    "role": {"type": "keyword"},
                    "name": {"type": "text"},
                    "content": {"type": "text", "analyzer": "standard"},
                    "topic": {"type": "keyword"},
                    "timestamp": {"type": "date"},
                    # Entity 字段
                    "entity_type": {"type": "keyword"},  # person, place, thing...
                    "description": {"type": "text"},
                    # Edge 字段
                    "source_id": {"type": "keyword"},
                    "target_id": {"type": "keyword"},
                    "relation": {"type": "keyword"},
                    "fact": {"type": "text"},
                    # Summary 字段
                    "episode_ids": {"type": "keyword"},
                    # 时间字段
                    "created_at": {"type": "date"},
                    "updated_at": {"type": "date"}
                }
            }
        }

        result = self._http_request(url, method="PUT", data=mapping)
        return result["success"]

    def opensearch_reset(self, index: str) -> bool:
        """Delete and recreate index"""
        self.opensearch_delete(index)
        return self.opensearch_create(index)

    # ========== Neo4j Operations ==========

    def neo4j_status(self) -> dict:
        """Check Neo4j status"""
        result = self._http_request(self.config.neo4j_url)
        if result["success"]:
            return {"online": True}
        return {"online": False, "error": result.get("error")}

    def neo4j_count(self) -> int:
        """Get total node count"""
        url = f"{self.config.neo4j_url}/db/neo4j/tx/commit"
        data = {"statements": [{"statement": "MATCH (n) RETURN count(n) as c"}]}
        auth = (self.config.neo4j_user, self.config.neo4j_pass)

        result = self._http_request(url, method="POST", data=data, auth=auth)
        if result["success"]:
            try:
                return result["data"]["results"][0]["data"][0]["row"][0]
            except (KeyError, IndexError):
                return 0
        return 0

    def neo4j_clear(self) -> bool:
        """Delete all nodes and relationships"""
        url = f"{self.config.neo4j_url}/db/neo4j/tx/commit"
        data = {"statements": [{"statement": "MATCH (n) DETACH DELETE n"}]}
        auth = (self.config.neo4j_user, self.config.neo4j_pass)

        result = self._http_request(url, method="POST", data=data, auth=auth)
        return result["success"]

    def neo4j_init(self) -> bool:
        """Create Neo4j indexes"""
        url = f"{self.config.neo4j_url}/db/neo4j/tx/commit"
        data = {
            "statements": [
                {"statement": "CREATE INDEX entity_name IF NOT EXISTS FOR (n:Entity) ON (n.name)"},
                {"statement": "CREATE INDEX entity_id IF NOT EXISTS FOR (n:Entity) ON (n.id)"},
                {"statement": "CREATE INDEX entity_agent_user IF NOT EXISTS FOR (n:Entity) ON (n.agent_id, n.user_id)"}
            ]
        }
        auth = (self.config.neo4j_user, self.config.neo4j_pass)

        result = self._http_request(url, method="POST", data=data, auth=auth)
        return result["success"]
