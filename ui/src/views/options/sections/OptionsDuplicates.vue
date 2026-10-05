<template>
  <div class="container">
    <b-loading :is-full-page="false" :active.sync="isLoading"></b-loading>
    <div class="content">
      <h3 class="title">{{$t('Duplicate scenes')}}</h3>
      <div class="card">
        <div class="card-content content">
          <p>
            {{$t('Scenes scraped twice under different sites — usually a second scraper added without master-site matching, so the alternate-source relink never attempts them. Review each group, keep one, or dismiss. Linking records the other as an alternate source; nothing is deleted and files stay where they are.')}}
          </p>
          <b-field grouped>
            <b-button class="button is-info" :loading="isLoading" @click="loadGroups">{{$t('Rescan')}}</b-button>
            <span style="margin-left: 1em; align-self: center;" v-if="!isLoading">{{ groups.length }} {{ $t('groups') }}</span>
          </b-field>
        </div>
      </div>
      <div class="card" v-for="group in groups" :key="group.key" style="margin-top: 1em;">
        <div class="card-content content">
          <div class="columns">
            <div class="column" v-for="scene in group.scenes" :key="scene.id">
              <p>
                <strong>{{ scene.title }}</strong><br>
                <small>{{ scene.site }} · {{ scene.scene_id }} · {{ scene.duration }} min<span v-if="scene.release_day"> · {{ scene.release_day }}</span></small><br>
                <small v-if="scene.file_count > 0">{{ scene.file_count }} {{ $t('matched file(s)') }}</small>
                <small v-else class="has-text-grey">{{ $t('no files') }}</small>
                <b-tag v-if="scene.id === group.suggested_keep" type="is-success is-light" size="is-small" style="margin-left: 0.4em;">{{ $t('suggested') }}: {{ group.suggest_why }}</b-tag>
              </p>
              <b-button class="button is-primary is-small is-outlined"
                        :loading="acting === linkKey(group, scene.id)"
                        @click="linkGroup(group, scene.id)">
                {{ $t('Keep this one') }}
              </b-button>
            </div>
          </div>
          <b-button class="button is-small is-text"
                    :loading="acting === 'dismiss:' + group.key"
                    @click="dismissGroup(group)">
            {{ $t('Not duplicates — dismiss') }}
          </b-button>
        </div>
      </div>
      <p v-if="!isLoading && groups.length === 0" class="has-text-grey">{{ $t('No duplicate groups found.') }}</p>
    </div>
  </div>
</template>

<script>
import ky from 'ky'

export default {
  name: 'OptionsDuplicates',
  data () {
    return {
      isLoading: true,
      groups: [],
      acting: ''
    }
  },
  mounted () {
    this.loadGroups()
  },
  methods: {
    async loadGroups () {
      this.isLoading = true
      try {
        const resp = await ky.get('/api/duplicates', { timeout: 120000 }).json()
        this.groups = resp.groups || []
      } catch (e) {
        this.groups = []
      }
      this.isLoading = false
    },
    linkKey (group, keepId) {
      return 'link:' + group.key + ':' + keepId
    },
    async linkGroup (group, keepId) {
      const loser = group.scenes.find(s => s.id !== keepId)
      if (!loser) return
      this.acting = this.linkKey(group, keepId)
      try {
        await ky.post('/api/duplicates/link', { json: { winner_id: keepId, loser_id: loser.id } }).json()
        this.groups = this.groups.filter(g => g.key !== group.key)
      } catch (e) {
      }
      this.acting = ''
    },
    async dismissGroup (group) {
      if (group.scenes.length < 2) return
      this.acting = 'dismiss:' + group.key
      try {
        await ky.post('/api/duplicates/dismiss', { json: { scene_a: group.scenes[0].id, scene_b: group.scenes[1].id } }).json()
        this.groups = this.groups.filter(g => g.key !== group.key)
      } catch (e) {
      }
      this.acting = ''
    }
  }
}
</script>
