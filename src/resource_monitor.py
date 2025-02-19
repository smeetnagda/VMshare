import psutil
import json

def get_system_resources():
    resources = {
        "cpu_percent": psutil.cpu_percent(interval=1),
        "cpu_cores": psutil.cpu_count(logical=False),
        "cpu_threads": psutil.cpu_count(logical=True),
        "memory_total": psutil.virtual_memory().total // (1024**2),  # in MB
        "memory_available": psutil.virtual_memory().available // (1024**2),  # in MB
        "disk_total": psutil.disk_usage('/').total // (1024**2),  # in MB
        "disk_available": psutil.disk_usage('/').free // (1024**2),  # in MB
        "network_speed": psutil.net_if_stats(),  # Raw data, can be processed further
    }
    return json.dumps(resources, indent=4)

if __name__ == "__main__":
    print(get_system_resources())

