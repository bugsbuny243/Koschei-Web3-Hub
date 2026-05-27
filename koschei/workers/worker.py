import os

WORKER_MAX_BUILD_THREADS = int(os.getenv("WORKER_MAX_BUILD_THREADS", "2"))

if __name__ == "__main__":
    print(f"Koschei build worker placeholder started (threads={WORKER_MAX_BUILD_THREADS})")
