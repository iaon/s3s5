package io.s3s5.android.objectstore

import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertArrayEquals
import org.junit.Assert.assertEquals
import org.junit.Test

class MemoryObjectStoreTest {
    @Test
    fun storesListsAndDeletesObjects() = runTest {
        val store = MemoryObjectStore()
        store.putObject("p/a", byteArrayOf(1))
        store.putObject("p/b", byteArrayOf(2, 3))
        store.putObject("q/c", byteArrayOf(4))

        assertArrayEquals(byteArrayOf(2, 3), store.getObject("p/b"))
        assertEquals(2, store.headObject("p/b").size)
        assertEquals(listOf("p/a", "p/b"), store.listPrefix("p/"))

        store.deleteObject("p/a")
        assertEquals(listOf("p/b"), store.listPrefix("p/"))
    }

    @Test
    fun listPrefixPageUsesContinuationToken() = runTest {
        val store = MemoryObjectStore()
        repeat(5) { i -> store.putObject("p/%02d".format(i), byteArrayOf(i.toByte())) }
        val first = store.listPrefixPage("p/", ListOptions(maxKeys = 2))
        assertEquals(listOf("p/00", "p/01"), first.keys)
        assertEquals(true, first.isTruncated)
        val second = store.listPrefixPage("p/", ListOptions(maxKeys = 2, continuationToken = first.nextContinuationToken))
        assertEquals(listOf("p/02", "p/03"), second.keys)
        assertEquals(true, second.isTruncated)
        val third = store.listPrefixPage("p/", ListOptions(maxKeys = 2, continuationToken = second.nextContinuationToken))
        assertEquals(listOf("p/04"), third.keys)
        assertEquals(false, third.isTruncated)
    }
}
