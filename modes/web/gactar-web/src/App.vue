<template>
  <span>
    <h1>
      <img src="/images/gactar-logo.svg" />
      gactar-web
      <span v-if="version" class="version-number">
        &nbsp;(<a href="https://github.com/asmaloney/gactar" target="_">
          {{ version }}
        </a>
        )
      </span>
    </h1>
    <div class="tile is-ancestor">
      <div class="tile is-vertical is-7 code-tile">
        <b-tabs
          v-model="activeTab"
          class="code-tabs"
          :animated="false"
          expanded
          :type="'is-boxed'"
        >
          <amod-code-tab
            :amod-issues="amodIssues"
            @codeChange="codeChange"
            @showError="showError"
          />

          <template v-for="tab in tabs">
            <framework-code-tab
              v-if="tab.displayed"
              :key="tab.framework.name"
              :value="tab.framework.name"
              :framework="tab.framework"
              :model-name="tab.modelName"
              :code="code[tab.framework.name]"
            >
            </framework-code-tab>
          </template>
        </b-tabs>
      </div>

      <div class="tile is-vertical is-parent">
        <div v-if="tabs.length > 0" class="tile is-child is-12">
          <b-field label="Select Frameworks" custom-class="is-small">
            <b-checkbox-button
              v-for="tab in tabs"
              v-model="selectedFrameworks"
              type="is-info"
              size="is-small"
              :native-value="tab.framework.name"
              :key="tab.framework.name"
              expanded
              class="ml-1 mr-1"
              @input="frameworkChanged"
            >
              <span>{{ tab.framework.name }}</span>
            </b-checkbox-button>
          </b-field>
        </div>

        <div class="tile is-child is-12">
          <b-field label="Goal" label-position="on-border">
            <b-input
              v-model="goal"
              placeholder="(initial goal here)"
              expanded
            />
            <p class="control">
              <b-button type="is-info" :loading="running" @click="run">
                <span class="fa fa-running icon-space" />Run
              </b-button>
            </p>
          </b-field>
        </div>

        <div class="tile is-child">
          <textarea id="results" v-model="results" expanded></textarea>
        </div>
      </div>
    </div>
  </span>
</template>

<script lang="ts">
import Vue from 'vue'

import api, {
  FrameworkInfo,
  FrameworkInfoList,
  FrameworkResultMap,
  IssueList,
  RunParams,
  RunResult,
  Version,
} from './api'

import { commentString, issuesToArray } from './utils'

import AmodCodeTab from './components/AmodCodeTab.vue'
import FrameworkCodeTab from './components/FrameworkCodeTab.vue'

interface Tab {
  framework: FrameworkInfo

  modelName: string
  displayed: boolean
}

type CodeMap = { [key: string]: string }
type FrameworkInfoMap = { [key: string]: FrameworkInfo }

interface Data {
  amodIssues: IssueList

  activeTab: number
  baseTabs: Tab[]
  code: CodeMap

  goal: string
  running: boolean
  results: string

  frameworks: FrameworkInfoMap
  availableFrameworks: string[]
  selectedFrameworks: string[]
  version: Version
}

const selectedFrameworksStorageName = 'gactar.selected-frameworks'

export default Vue.extend({
  components: { AmodCodeTab, FrameworkCodeTab },

  data(): Data {
    return {
      amodIssues: [],

      activeTab: 0,
      baseTabs: [],

      code: {},
      goal: '',
      running: false,
      results: '',

      frameworks: {},
      availableFrameworks: [],
      selectedFrameworks: [],
      version: '',
    }
  },

  computed: {
    tabs(): Tab[] {
      return this.baseTabs
    },
  },

  created() {
    window.addEventListener('load', () => {
      this.onWindowLoad()
    })
  },

  mounted() {
    this.loadFrameworks()
    this.loadVersion()
  },

  methods: {
    frameworkChanged() {
      // Save our selected frameworks
      localStorage.setItem(
        selectedFrameworksStorageName,
        JSON.stringify(this.selectedFrameworks)
      )
    },

    clearResults() {
      this.results = ''
    },

    codeChange(newCode: string) {
      this.code['amod'] = newCode
    },

    hideTabsNotInUse() {
      this.baseTabs.forEach((tab: Tab) => {
        if (!this.selectedFrameworks.includes(tab.framework.name)) {
          tab.displayed = false
        }
      })
    },

    loadFrameworks() {
      api
        .getFrameworks()
        .then((list: FrameworkInfoList) => {
          list.forEach((info: FrameworkInfo) => {
            // create tab info for each language present on the server
            const tab: Tab = {
              framework: info,
              modelName: '',
              displayed: false,
            }

            this.frameworks[info.name] = info
            this.availableFrameworks.push(info.name)

            this.baseTabs.push(tab)
          })
        })
        .catch((err: Error) => {
          this.showError(err.message)
        })
    },

    loadVersion() {
      api
        .getVersion()
        .then((version: Version) => {
          this.version = version
        })
        .catch((err: Error) => {
          this.showError(err.message)
        })
    },

    onWindowLoad() {
      // Load our selected frameworks from local storage (if any)
      var frameworks = localStorage.getItem(selectedFrameworksStorageName)
      if (frameworks === null) {
        this.selectedFrameworks = this.availableFrameworks
      } else {
        this.selectedFrameworks = JSON.parse(frameworks) as string[]

        // Filter the saved list by the available frameworks.
        const availableFrameworks = this.availableFrameworks // need this const because we can't use "this" inside filter
        this.selectedFrameworks = this.selectedFrameworks.filter(function (
          name: string
        ) {
          return availableFrameworks.includes(name)
        })
      }

      window.removeEventListener('load', () => {
        this.onWindowLoad()
      })
    },

    run() {
      this.running = true

      this.clearResults()
      this.hideTabsNotInUse()

      const params: RunParams = {
        amod: this.code['amod'],
        goal: this.goal,
        frameworks: this.selectedFrameworks,
      }

      api
        .run(params)
        .then((result: RunResult) => {
          if (result.issues) {
            this.showIssues(result.issues)
          }
          if (result.results) {
            this.setResults(result.results)
          }
          this.running = false
        })
        .catch((err: Error) => {
          this.showError(err.message)
          this.running = false
        })
    },

    setResults(results: FrameworkResultMap) {
      let text = ''
      for (const [framework, result] of Object.entries(results)) {
        text += framework + '\n' + '---\n'

        if (result.issues) {
          const issueTexts = issuesToArray(result.issues)
          text += issueTexts.join('\n') + '\n\n'
        }

        if (result.output) {
          text += result.output
          text += '\n\n'
        }

        if (result.code) {
          this.code[framework] = result.code
        } else {
          this.code[framework] = commentString(
            this.frameworks[framework].language,
            '(No code returned from server)'
          )
        }

        const index = this.tabs.findIndex(
          (obj: Tab) => obj.framework.name == framework
        )
        if (index != -1) {
          this.tabs[index].modelName = result.modelName

          // show our tabs the first time we have code
          if (this.code[framework] && this.code[framework].length != 0) {
            this.tabs[index].displayed = true
          }
        }
      }

      this.results += text
    },

    showError(err: string) {
      this.results = err
    },

    showIssues(list: IssueList) {
      Vue.set(this, 'amodIssues', list)

      const issueTexts = issuesToArray(list)

      this.results += issueTexts.join('\n') + '\n\n'
    },
  },
})
</script>
