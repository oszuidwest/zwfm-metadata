const ICECAST_SERVERS = [
  { hostName: 'icecast.zuidwest.cloud', port: 80, username: 'admin', password: 'hackme', mountPoint: '/zuidwest.mp3' },
  { hostName: 'icecast.zuidwest.cloud', port: 80, username: 'admin', password: 'hackme', mountPoint: '/zuidwest.aac' }
  // Add more servers if needed
]

// Configuration for the ODR Web API
const ODR_API = {
  hostName: 'localhost',
  port: 9000,
  authToken: 'verySecretToken'
}

export default {
  async scheduled (event, env, ctx) {
    // Fetch current playing song
    const response = await fetch('https://rds.zuidwestfm.nl/')
    const song = await response.text()

    // Push to Icecast servers
    const errors = []
    for (const server of ICECAST_SERVERS) {
      const metadata = { song, mount: server.mountPoint, mode: 'updinfo', charset: 'UTF-8' }
      const requestOptions = {
        method: 'GET',
        headers: {
          Authorization: 'Basic ' + btoa(server.username + ':' + server.password)
        }
      }

      const url = `http://${server.hostName}:${server.port}/admin/metadata.xsl?` + new URLSearchParams(metadata)
      const serverResponse = await fetch(url, requestOptions)

      if (!serverResponse.ok) {
        errors.push(`Error ${serverResponse.status}: ${serverResponse.statusText} on ${server.hostName}`)
      }
    }

    // Push to ODR Web API
    try {
      const odrRequestOptions = {
        method: 'POST',
        headers: {
          Authorization: ODR_API.authToken,
          'Content-Type': 'text/plain'
        },
        body: song
      }

      const odrApiUrl = `http://${ODR_API.hostName}:${ODR_API.port}/api/dls`
      const odrResponse = await fetch(odrApiUrl, odrRequestOptions)

      if (!odrResponse.ok) {
        const errorText = await odrResponse.text()
        errors.push(`Error ${odrResponse.status}: ${errorText} when pushing to ODR API`)
      } else {
        console.log('Successfully pushed metadata to ODR API')
      }
    } catch (error) {
      errors.push(`Exception when pushing to ODR API: ${error.message}`)
    }

    if (errors.length > 0) {
      console.error('One or more requests failed: \n' + errors.join('\n'))
      return
    }

    console.log('Updated song metadata on all platforms')
  }
}
