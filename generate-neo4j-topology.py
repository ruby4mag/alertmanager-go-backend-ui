from neo4j import GraphDatabase
from faker import Faker
import random
import uuid

fake = Faker()

URI = "bolt://localhost:7687"
USER = "neo4j"
PASSWORD = "password"

driver = GraphDatabase.driver(URI, auth=(USER, PASSWORD))

# ---------- CONFIG ----------
DATACENTERS = 2
RACKS_PER_DC = 20
HOSTS_PER_RACK = 20
VMS_PER_HOST = 10
APPS_PER_VM = 0.5   # average
K8S_NODES = 1000
PODS_PER_NODE = 4

# ---------- HELPERS ----------
def run(tx, query, params=None):
    tx.run(query, params or {})

# ---------- CREATE BASE ENTITIES ----------
def create_base(tx):
    tx.run("CREATE CONSTRAINT IF NOT EXISTS FOR (n:Datacenter) REQUIRE n.id IS UNIQUE")
    tx.run("CREATE CONSTRAINT IF NOT EXISTS FOR (n:Host) REQUIRE n.id IS UNIQUE")
    tx.run("CREATE CONSTRAINT IF NOT EXISTS FOR (n:VM) REQUIRE n.id IS UNIQUE")
    tx.run("CREATE CONSTRAINT IF NOT EXISTS FOR (n:Application) REQUIRE n.id IS UNIQUE")
    tx.run("CREATE CONSTRAINT IF NOT EXISTS FOR (n:K8sNode) REQUIRE n.id IS UNIQUE")

# ---------- DATACENTERS ----------
def create_datacenters(tx):
    for i in range(DATACENTERS):
        tx.run("""
        CREATE (:Datacenter {id:$id, name:$name, region:$region})
        """, {
            "id": f"dc-{i}",
            "name": f"DC-{i}",
            "region": random.choice(["APAC", "EMEA", "US"])
        })

# ---------- RACKS + HOSTS ----------
def create_racks_hosts(tx):
    for dc in range(DATACENTERS):
        for r in range(RACKS_PER_DC):
            rack_id = f"rack-{dc}-{r}"
            tx.run("""
            MATCH (d:Datacenter {id:$dc})
            CREATE (r:Rack {id:$rid})
            CREATE (d)-[:HAS_RACK]->(r)
            """, {"dc": f"dc-{dc}", "rid": rack_id})

            for h in range(HOSTS_PER_RACK):
                host_id = f"host-{dc}-{r}-{h}"
                tx.run("""
                MATCH (r:Rack {id:$rid})
                CREATE (h:Host {
                    id:$hid,
                    vendor:$vendor,
                    cpu:$cpu,
                    memory:$mem
                })
                CREATE (r)-[:HAS_HOST]->(h)
                """, {
                    "rid": rack_id,
                    "hid": host_id,
                    "vendor": random.choice(["Dell", "HPE", "Lenovo"]),
                    "cpu": random.choice([32, 48, 64]),
                    "mem": random.choice([128, 256, 512])
                })

# ---------- HYPERVISORS ----------
def create_hypervisors(tx):
    for hv in ["VMware", "Hyper-V"]:
        tx.run("""
        CREATE (:Hypervisor {name:$name})
        """, {"name": hv})

    tx.run("""
    MATCH (h:Host), (v:Hypervisor)
    WHERE rand() < 0.5
    CREATE (h)-[:RUNS_ON]->(v)
    """)

# ---------- VMS ----------
def create_vms(tx):
    tx.run("""
    MATCH (h:Host)
    WITH h LIMIT 800
    UNWIND range(1,$vms) AS i
    CREATE (v:VM {
        id: apoc.create.uuid(),
        os: random.choice(["Linux","Windows"]),
        cpu: random.choice([2,4,8]),
        memory: random.choice([4,8,16])
    })
    CREATE (h)-[:HOSTS_VM]->(v)
    """, {"vms": VMS_PER_HOST})

# ---------- APPLICATIONS ----------
def create_apps(tx):
    tx.run("""
    MATCH (v:VM)
    WHERE rand() < $ratio
    CREATE (a:Application {
        id: apoc.create.uuid(),
        name: $name,
        tier: random.choice(["frontend","backend","db"])
    })
    CREATE (v)-[:RUNS_APP]->(a)
    """, {"ratio": APPS_PER_VM, "name": fake.company()})

# ---------- STORAGE ----------
def create_storage(tx):
    for i in range(10):
        tx.run("CREATE (:StorageArray {id:$id, vendor:$v})",
               {"id": f"storage-{i}", "v": random.choice(["NetApp","EMC","Pure"])})
    for i in range(40):
        tx.run("CREATE (:SanSwitch {id:$id})", {"id": f"san-{i}"})

    tx.run("""
    MATCH (s:SanSwitch),(a:StorageArray)
    WHERE rand() < 0.3
    CREATE (s)-[:CONNECTED_TO]->(a)
    """)

    tx.run("""
    MATCH (h:Host),(s:SanSwitch)
    WHERE rand() < 0.2
    CREATE (h)-[:CONNECTED_TO]->(s)
    """)

# ---------- NETWORK ----------
def create_network(tx):
    for i in range(120):
        tx.run("""
        CREATE (:NetworkDevice {
            id:$id,
            type:$type
        })
        """, {
            "id": f"net-{i}",
            "type": random.choice(["Core","Distribution","Access"])
        })

    tx.run("""
    MATCH (a:NetworkDevice),(b:NetworkDevice)
    WHERE rand() < 0.05 AND a <> b
    CREATE (a)-[:CONNECTED_TO]->(b)
    """)

    tx.run("""
    MATCH (h:Host),(n:NetworkDevice)
    WHERE rand() < 0.1
    CREATE (h)-[:CONNECTED_TO]->(n)
    """)

# ---------- KUBERNETES ----------
def create_k8s(tx):
    tx.run("CREATE (:K8sCluster {id:'cluster-1'})")

    for i in range(K8S_NODES):
        tx.run("""
        MATCH (c:K8sCluster)
        CREATE (n:K8sNode {id:$id})
        CREATE (c)-[:HAS_NODE]->(n)
        """, {"id": f"k8s-node-{i}"})

    tx.run("""
    MATCH (n:K8sNode)
    UNWIND range(1,$pods) AS i
    CREATE (p:Pod {id:apoc.create.uuid()})
    CREATE (n)-[:RUNS_POD]->(p)
    """, {"pods": PODS_PER_NODE})

    tx.run("""
    MATCH (p:Pod)
    WHERE rand() < 0.3
    CREATE (s:Service {id:apoc.create.uuid()})
    CREATE (p)-[:EXPOSES_SERVICE]->(s)
    """)

    tx.run("""
    MATCH (v:VM),(n:K8sNode)
    WHERE rand() < 0.2
    CREATE (v)-[:RUNS_NODE]->(n)
    """)

# ---------- EXECUTION ----------
with driver.session() as session:
    session.execute_write(create_base)
    session.execute_write(create_datacenters)
    session.execute_write(create_racks_hosts)
    session.execute_write(create_hypervisors)
    session.execute_write(create_vms)
    session.execute_write(create_apps)
    session.execute_write(create_storage)
    session.execute_write(create_network)
    session.execute_write(create_k8s)

print("Enterprise Neo4j topology created successfully 🚀")

