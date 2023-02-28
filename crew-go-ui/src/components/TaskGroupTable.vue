<template>
  <div class="full-width">
    <q-table
      :title="props.title"
      :rows="taskGroups"
      :columns="columns"
      v-model:pagination="paginationModel"
      row-key="id"
      :loading="loading"
      :flat="props.embedded"
      @request="onRequest"
      class="full-width"
      wrap-cells
    >
      <template v-slot:top>
        <div class="row full-width text-center" :class="{'text-left': !$q.screen.xs}">
          <div v-if="props.title" class="col-12 col-sm-3">
            <div class="q-table__title">
              {{ props.title }}
            </div>
          </div>

          <div :class="{
            'q-mt-sm': $q.screen.xs,
            'col-12 col-sm-5': props.title && !hideCreateButtons,
            'col-12 col-sm-9': props.title && hideCreateButtons,
            'col-12 col-sm-8': !props.title && !hideCreateButtons,
            'col-12': !props.title && props.hideCreateButtons
          }">
            <q-input
              type="text"
              v-model="search"
              placeholder="Search"
              filled
              dense
              @keyup.enter="loadTaskGroups"
            >
              <template v-slot:after>
                <q-btn round dense flat icon="search" @click="loadTaskGroups" />
                <q-btn round dense flat icon="clear" @click="clearSearch" />
              </template>
            </q-input>
          </div>
          <div v-if="!props.hideCreateButtons" class="col-12 col-sm-4 text-center" :class="{'text-right': !$q.screen.xs, 'q-mt-sm': $q.screen.xs}">
            <CreateTaskGroupModalButton @on-create="onCreate" />
          </div>
        </div>
      </template>

      <template v-slot:body="props">
        <q-tr :props="props" valign="top">
          <q-td key="id" :props="props">
            <q-btn icon="content_copy" size="sm" @click="toClipboard('Id', props.row.id)" :label="props.row.id" />
          </q-td>

          <q-td key="name" :props="props">
            {{ props.row.name }}
          </q-td>

          <q-td key="actions" :props="props">
            <q-btn :to="{ name: 'task_group', params: { taskGroupId: props.row.id } }" color="purple" size="sm" icon="launch" />
          </q-td>

        </q-tr>
      </template>
    </q-table>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import toClipboard from 'src/lib/toClipboard'
import { useRouter, useRoute } from 'vue-router'
import notifyError from 'src/lib/notifyError'
import CreateTaskGroupModalButton from 'src/components/task-group/CreateTaskGroupModalButton.vue'
import { QTableProps } from 'quasar'
import { useTaskGroupStore, TaskGroup } from 'src/stores/task-group-store'

const taskGroupStore = useTaskGroupStore()

const router = useRouter()
const route = useRoute()

const props = defineProps({
  title: {
    type: String,
    required: false,
    default: 'Task Groups'
  },
  hideCreateButtons: {
    type: Boolean,
    required: false,
    default: false
  },
  embedded: {
    type: Boolean,
    required: false,
    default: false
  }
})

const loading = ref(true)
const taskGroups = ref<Array<TaskGroup>>([])
const paginationModel = ref<QTableProps['pagination']>({
  page: parseInt(router.currentRoute.value.query.page ? router.currentRoute.value.query.page as string : '1'),
  rowsPerPage: router.currentRoute.value.query.page_size ? parseInt(router.currentRoute.value.query.page_size as string) : 20,
  rowsNumber: 0
})
const search = ref(router.currentRoute.value.query.q as string || '')

const columns : QTableProps['columns'] = [
  {
    name: 'id',
    field: 'id',
    label: 'Id',
    align: 'center'
  },
  {
    name: 'name',
    field: 'name',
    label: 'Name',
    align: 'left'
  },
  {
    name: 'actions',
    field: '',
    label: 'Actions',
    align: 'left'
  }
]

function onCreate (group: TaskGroup) {
  router.push({ name: 'task_group', params: { taskGroupId: group.id } })
}

async function clearSearch () {
  search.value = ''
  await loadTaskGroups()
}

const onRequest : QTableProps['onRequest'] = async ({ pagination }) => {
  paginationModel.value = pagination
  await loadTaskGroups()
}

async function loadTaskGroups () {
  try {
    if (paginationModel.value) {
      loading.value = true
      const result = await taskGroupStore.getTaskGroups(paginationModel.value.page, paginationModel.value.rowsPerPage, search.value)
      paginationModel.value.rowsNumber = result.count
      taskGroups.value = result.taskGroups
      router.push({ query: { ...route.query, q: search.value, page: paginationModel.value.page, page_size: paginationModel.value.rowsPerPage } })
    }
  } catch (e) {
    notifyError(e)
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await loadTaskGroups()
})
</script>
