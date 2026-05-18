class TransactionBatcher {
  constructor() {
    this.queue = [];
    this.flushInterval = 2000;
    this.maxBatchSize = 10;
    setInterval(() => this.flush(), this.flushInterval).unref();
  }

  async add(txFn) {
    return new Promise((resolve, reject) => {
      this.queue.push({ txFn, resolve, reject });
      if (this.queue.length >= this.maxBatchSize) {
        this.flush().catch((err) => reject(err));
      }
    });
  }

  async flush() {
    if (this.queue.length === 0) return;
    const batch = this.queue.splice(0, this.maxBatchSize);
    await Promise.allSettled(
      batch.map((item) => item.txFn().then(item.resolve).catch(item.reject))
    );
  }
}

module.exports = new TransactionBatcher();
