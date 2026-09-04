// S3-compatible object storage facade. The `client` is an injected port so this
// stays SDK-free and testable: production wires the S3 adapter (MinIO locally,
// real S3/GCS in prod — same API, endpoint from env); tests inject a fake.
//
// client.send(descriptor) where descriptor.op is 'PutObject' | 'GetObject'.

export function createStorage({ client, bucket, publicBase = '' }) {
  if (!bucket) throw new Error('bucket is required')
  return {
    async put(key, body, contentType = 'application/octet-stream') {
      await client.send({ op: 'PutObject', Bucket: bucket, Key: key, Body: body, ContentType: contentType })
      return { key, url: publicUrl(publicBase, bucket, key) }
    },
    async get(key) {
      const res = await client.send({ op: 'GetObject', Bucket: bucket, Key: key })
      return res.Body
    },
    publicUrl(key) {
      return publicUrl(publicBase, bucket, key)
    },
  }
}

function publicUrl(base, bucket, key) {
  return `${base.replace(/\/$/, '')}/${bucket}/${key}`
}
