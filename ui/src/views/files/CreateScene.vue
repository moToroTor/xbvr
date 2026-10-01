<template>
  <div class="modal is-active">
    <div class="modal-background"></div>
    <div class="modal-card">
      <header class="modal-card-head">
        <p class="modal-card-title">{{ $t("Create Custom Scene") }}</p>
        <button class="delete" @click="close" aria-label="close"></button>
      </header>
      <section class="modal-card-body">
        <div>
          <h6 class="title is-6">{{ file.filename }}</h6>
          <small>
            <span class="pathDetails">{{ file.path }}</span>
            <br/>
            {{ prettyBytes(file.size) }}, {{ file.video_width }}x{{ file.video_height }},
            {{ format(parseISO(file.created_time), "yyyy-MM-dd") }}
          </small>
          <b-field :label="$t('Scene Id')" label-position="on-border" grouped>
            <b-tooltip label="If blank a Scene Id will be generated but cannot be changed later"  :delay="500" >
              <b-input v-model="sceneId" placeholder="Can be empty" ref="sceneIdInput"></b-input>
            </b-tooltip>
          </b-field>
          <b-field :label="$t('Title')" label-position="on-border">
            <b-input v-model='title' ></b-input>
          </b-field>
          <hr>
          <h6 class="title is-6">{{ $t("Search the internet") }}</h6>
          <p class="is-size-7" style="margin-bottom:0.75em">{{ $t("Look for a page on a site XBVR can scrape before creating a manual scene.") }}</p>
          <b-field grouped>
            <b-input v-model='webQuery' expanded></b-input>
            <b-button class="button is-info" :loading="searching" v-on:click="searchWeb()">{{$t('Search')}}</b-button>
          </b-field>
          <b-notification v-if="webError" type="is-danger is-light" :closable="false" style="margin-bottom:0.75em">{{ webError }}</b-notification>
          <b-table v-if="candidates.length > 0" :data="candidates" :show-header="false" narrowed>
            <b-table-column field="source" v-slot="props">
              <strong>{{ props.row.scraper_name }}</strong>
              <b-tag v-if="props.row.preferred" type="is-success is-light" size="is-small" style="margin-left:0.4em">{{ $t("Preferred") }}</b-tag>
              <br>
              <small><a :href="props.row.url" target="_blank" rel="noreferrer">{{ props.row.title || props.row.url }}</a></small>
              <br>
              <small class="has-text-grey">{{ props.row.reason }}</small>
            </b-table-column>
            <b-table-column field="action" numeric v-slot="props">
              <b-button class="button is-primary is-small is-outlined" :loading="scrapingUrl === props.row.url" v-on:click="scrapeAndMatch(props.row)">{{ $t("Scrape & Match") }}</b-button>
            </b-table-column>
          </b-table>
          <p v-else-if="webSearched" class="is-size-7 has-text-grey" style="margin-bottom:0.75em">{{ $t("No pages on supported sites found. Try fewer words or create the scene manually.") }}</p>
          <hr>
          <b-button class="button is-primary" style="margin-right:1em" v-on:click="addScene(false)">{{$t('Create')}}</b-button>
          <b-button class="button is-primary" v-on:click="addScene(true)">{{$t('Create and Edit')}} </b-button>
        </div>
      </section>
    </div>
  </div>
</template>

<script>
import ky from 'ky'
import { format, parseISO } from 'date-fns'
import prettyBytes from 'pretty-bytes'

export default {
  name: 'CreateScene',  
  data () {
    return {
      title: '',
      sceneId: '',
      webQuery: '',
      searching: false,
      webSearched: false,
      candidates: [],
      webError: '',
      scrapingUrl: '',
      format,
      parseISO
    }
  },
  computed: {
    file () {
      return this.$store.state.overlay.createScene.file
    }
  },
  mounted () {
    this.initView()
  },
  methods: {
    initView () {
      const commonWords = [
        '180', '180x180', '2880x1440', '3d', '3dh', '3dv', '30fps', '30m', '360',
        '3840x1920', '4k', '5k', '5400x2700', '60fps', '6k', '7k', '7680x3840',
        '8k', 'fb360', 'fisheye190', 'funscript', 'h264', 'h265', 'hevc', 'hq', 'hsp', 'lq', 'lr',
        'mkv', 'mkx200', 'mkx220', 'mono', 'mp4', 'oculus', 'oculus5k',
        'oculusrift', 'original', 'rf52', 'smartphone', 'srt', 'ssa', 'tb', 'uhq', 'vrca220', 'vp9'
      ]
      const isNotCommonWord = word => !commonWords.includes(word.toLowerCase()) && !/^[0-9]+p$/.test(word)

      this.title = (
        this.file.filename
          .replace(/\.|_|\+|-/g, ' ').replace(/\s+/g, ' ').trim()
          .split(' ').filter(isNotCommonWord).join(' ')
          .replace(/ s /g, '\'s '))
      this.webQuery = this.title
      this.$refs.sceneIdInput.focus()
    },
    close () {
      this.$store.commit('overlay/hideCreateCustomScene')
    },
    toInt (value, radix, defaultValue) {
      return parseInt(value, radix || 10) || defaultValue || 0
    },
    searchWeb () {
      this.searching = true
      this.webError = ''
      ky.post('/api/task/scrape-search', { json: { q: this.webQuery }, timeout: 60000 })
        .json()
        .then(resp => {
          this.candidates = resp.candidates || []
          this.webSearched = true
          this.searching = false
        })
        .catch(() => {
          this.webError = 'Search failed. Check your connection and try again.'
          this.searching = false
        })
    },
    scrapeAndMatch (candidate) {
      this.scrapingUrl = candidate.url
      this.webError = ''
      ky.post('/api/task/singlescrape', { timeout: false, json: { site: candidate.scraper_id, sceneurl: candidate.url, additionalinfo: [] } })
        .json()
        .then(resp => {
          if (!resp.scene || !resp.scene.scene_id) {
            this.webError = 'The scrape returned no scene. Try another result or create the scene manually.'
            this.scrapingUrl = ''
            return
          }
          ky.post('/api/files/match', { json: { file_id: this.file.id, scene_id: resp.scene.scene_id } })
            .then(() => {
              this.$store.dispatch('files/load')
              this.scrapingUrl = ''
              this.close()
            })
        })
        .catch(() => {
          this.webError = 'The scrape failed. Try another result or create the scene manually.'
          this.scrapingUrl = ''
        })
    },
    addScene(showEdit) {
      ky.post('/api/scene/create', { json: { title: this.title, id: this.sceneId, filename: this.file.filename } })
        .json()
        .then(scene => {          
          ky.post('/api/files/match', { json: {file_id: this.file.id, scene_id: scene.scene_id}})          
          .then(data => {
            this.$store.dispatch('files/load')
            this.close()
            if (showEdit) {
              this.$store.commit('overlay/editDetails', { scene: scene })
            }
          })          
        })
    },
    prettyBytes
  }
}
</script>

<style scoped>
h6.title.is-6 {
  margin-bottom: 0;
}

h6 + small {
  margin-bottom: 1.5rem;
  display: inline-block;
  font-size: small;
}

h6 + small > .pathDetails {
  color: #B0B0B0;
}

.modal-card {
  position: absolute;
  top: 4em;
  width: 80%;
}

</style>
