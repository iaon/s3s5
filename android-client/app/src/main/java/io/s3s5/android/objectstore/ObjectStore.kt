package io.s3s5.android.objectstore

import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import java.util.TreeMap
import java.util.concurrent.atomic.AtomicLong

class ObjectNotFoundException(key: String) : Exception("object not found: $key")

data class PutOptions(
    val contentType: String = "",
    val metadata: Map<String, String> = emptyMap(),
)

data class ListOptions(
    val maxKeys: Int = 0,
)

data class ObjectInfo(
    val key: String,
    val size: Long,
    val metadata: Map<String, String> = emptyMap(),
)

interface ObjectStore {
    suspend fun putObject(key: String, data: ByteArray, options: PutOptions = PutOptions())
    suspend fun getObject(key: String): ByteArray
    suspend fun headObject(key: String): ObjectInfo
    suspend fun listPrefix(prefix: String, options: ListOptions = ListOptions()): List<String>
    suspend fun deleteObject(key: String)
}

data class StoreCounters(
    val put: Long,
    val get: Long,
    val head: Long,
    val list: Long,
    val delete: Long,
)

class CountingObjectStore(private val delegate: ObjectStore) : ObjectStore {
    private val putCount = AtomicLong()
    private val getCount = AtomicLong()
    private val headCount = AtomicLong()
    private val listCount = AtomicLong()
    private val deleteCount = AtomicLong()

    fun snapshot(): StoreCounters = StoreCounters(
        put = putCount.get(),
        get = getCount.get(),
        head = headCount.get(),
        list = listCount.get(),
        delete = deleteCount.get(),
    )

    override suspend fun putObject(key: String, data: ByteArray, options: PutOptions) {
        putCount.incrementAndGet()
        delegate.putObject(key, data, options)
    }

    override suspend fun getObject(key: String): ByteArray {
        getCount.incrementAndGet()
        return delegate.getObject(key)
    }

    override suspend fun headObject(key: String): ObjectInfo {
        headCount.incrementAndGet()
        return delegate.headObject(key)
    }

    override suspend fun listPrefix(prefix: String, options: ListOptions): List<String> {
        listCount.incrementAndGet()
        return delegate.listPrefix(prefix, options)
    }

    override suspend fun deleteObject(key: String) {
        deleteCount.incrementAndGet()
        delegate.deleteObject(key)
    }
}

class MemoryObjectStore : ObjectStore {
    private data class Entry(val data: ByteArray, val metadata: Map<String, String>)

    private val mutex = Mutex()
    private val objects = TreeMap<String, Entry>()

    override suspend fun putObject(key: String, data: ByteArray, options: PutOptions) {
        mutex.withLock {
            objects[key] = Entry(data.copyOf(), options.metadata.toMap())
        }
    }

    override suspend fun getObject(key: String): ByteArray = mutex.withLock {
        objects[key]?.data?.copyOf() ?: throw ObjectNotFoundException(key)
    }

    override suspend fun headObject(key: String): ObjectInfo = mutex.withLock {
        val entry = objects[key] ?: throw ObjectNotFoundException(key)
        ObjectInfo(key = key, size = entry.data.size.toLong(), metadata = entry.metadata)
    }

    override suspend fun listPrefix(prefix: String, options: ListOptions): List<String> = mutex.withLock {
        val keys = objects.keys
            .asSequence()
            .filter { it.startsWith(prefix) }
            .sorted()
        if (options.maxKeys > 0) {
            keys.take(options.maxKeys).toList()
        } else {
            keys.toList()
        }
    }

    override suspend fun deleteObject(key: String) {
        mutex.withLock {
            objects.remove(key)
        }
    }
}
