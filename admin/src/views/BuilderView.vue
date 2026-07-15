<template>
  <a-card class="panel" :bordered="false">
    <template #title>Custom Client Builder</template>
    <template #extra>
      <a-space>
        <a-button @click="load">Refresh</a-button>
        <a-button :loading="checkingDoctor" @click="refreshDoctor">Check Environment</a-button>
        <a-button type="primary" :loading="creatingProfile" @click="createDefaultProfile">Create Profile</a-button>
        <a-button :disabled="!profiles.length" :loading="queueingJob" @click="queueWindowsJob">Queue Windows Job</a-button>
        <a-button :disabled="!jobs.some((job) => job.status === 'queued')" :loading="runningJob" @click="runNextJob">Run Next</a-button>
      </a-space>
    </template>
    <a-space direction="vertical" fill>
      <a-alert v-if="message" :type="messageType">{{ message }}</a-alert>
      <a-spin :loading="loading" style="width: 100%">
        <a-typography-title :heading="6">Runner Status</a-typography-title>
        <a-table :columns="platformColumns" :data="platforms" :pagination="false" row-key="platform" />
        <a-divider />
        <a-space align="center">
          <a-typography-title :heading="6" style="margin: 0">Build Environment</a-typography-title>
          <a-tag v-if="doctorReport" :color="doctorReport.ready ? 'green' : 'red'">
            {{ doctorReport.ready ? 'Ready' : 'Needs setup' }}
          </a-tag>
        </a-space>
        <a-typography-text v-if="!doctorReport && checkingDoctor" type="secondary">
          Checking Windows build environment...
        </a-typography-text>
        <a-typography-text v-else-if="!doctorReport" type="secondary">
          Environment status has not loaded yet.
        </a-typography-text>
        <a-table
          v-if="doctorReport"
          :columns="doctorColumns"
          :data="doctorRows"
          :pagination="false"
          row-key="name"
        />
        <a-divider />
        <a-typography-title :heading="6">Build Profiles</a-typography-title>
        <a-table
          :columns="profileColumns"
          :data="profiles"
          :pagination="{ pageSize: 5 }"
          row-key="id"
        />
        <a-divider />
        <a-typography-title :heading="6">Build Jobs</a-typography-title>
        <a-table
          :columns="jobColumns"
          :data="jobs"
          :pagination="{ pageSize: 8 }"
          row-key="id"
        >
          <template #actions="{ record }">
            <a-space>
              <a-button
                size="mini"
                :disabled="record.status !== 'queued'"
                :loading="jobActionId === record.id && jobAction === 'cancel'"
                @click="cancelJob(record)"
              >
                Cancel
              </a-button>
              <a-button
                size="mini"
                :disabled="record.status === 'queued' || record.status === 'running'"
                :loading="jobActionId === record.id && jobAction === 'retry'"
                @click="retryJob(record)"
              >
                Retry
              </a-button>
              <a-button
                size="mini"
                :loading="jobActionId === record.id && jobAction === 'artifacts'"
                @click="loadArtifacts(record)"
              >
                Artifacts
              </a-button>
            </a-space>
          </template>
        </a-table>
        <a-space v-if="artifacts.length" wrap>
          <a-button
            v-for="artifact in artifacts"
            :key="artifact.id"
            size="small"
            @click="downloadArtifact(artifact)"
          >
            {{ artifact.filename }}
          </a-button>
        </a-space>
      </a-spin>
    </a-space>
  </a-card>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { apiDownload, apiGet, apiPost, humanError } from '../api/client';

interface BuildProfile {
  id: number;
  name: string;
  app_name: string;
  bundle_id: string;
  product_name: string;
  created_at: string;
}

interface BuildJob {
  id: number;
  profile_id: number;
  platform: string;
  status: string;
  runner: string;
  error_message?: string;
  created_at: string;
}

interface BuildArtifact {
  id: number;
  build_job_id: number;
  platform: string;
  filename: string;
  local_path: string;
  sha256: string;
  size_bytes: number;
}

interface DoctorCheck {
  name: string;
  required: boolean;
  status: string;
  detail: string;
  path?: string;
}

interface DoctorReport {
  platform: string;
  source_dir: string;
  ready: boolean;
  checks: DoctorCheck[];
}

const platforms = [
  { platform: 'Windows x64', status: 'staging and command runner' },
  { platform: 'macOS x64', status: 'runner interface' },
  { platform: 'macOS arm64', status: 'runner interface' },
  { platform: 'Android arm64', status: 'runner interface' },
  { platform: 'iOS arm64', status: 'capability matrix required' }
];
const platformColumns = [
  { title: 'Platform', dataIndex: 'platform' },
  { title: 'Status', dataIndex: 'status' }
];
const profileColumns = [
  { title: 'ID', dataIndex: 'id' },
  { title: 'Name', dataIndex: 'name' },
  { title: 'App', dataIndex: 'app_name' },
  { title: 'Bundle ID', dataIndex: 'bundle_id' },
  { title: 'Created', dataIndex: 'created_at' }
];
const jobColumns = [
  { title: 'ID', dataIndex: 'id' },
  { title: 'Profile', dataIndex: 'profile_id' },
  { title: 'Platform', dataIndex: 'platform' },
  { title: 'Status', dataIndex: 'status' },
  { title: 'Runner', dataIndex: 'runner' },
  { title: 'Created', dataIndex: 'created_at' },
  { title: 'Actions', slotName: 'actions' }
];
const doctorColumns = [
  { title: 'Check', dataIndex: 'name' },
  { title: 'Required', dataIndex: 'required' },
  { title: 'Status', dataIndex: 'status' },
  { title: 'Detail', dataIndex: 'detail' },
  { title: 'Path', dataIndex: 'path' }
];
const loading = ref(false);
const creatingProfile = ref(false);
const queueingJob = ref(false);
const runningJob = ref(false);
const checkingDoctor = ref(false);
const jobActionId = ref<number | null>(null);
const jobAction = ref<'cancel' | 'retry' | 'artifacts' | 'download' | ''>('');
const message = ref('');
const messageType = ref<'info' | 'success' | 'warning' | 'error'>('info');
const profiles = ref<BuildProfile[]>([]);
const jobs = ref<BuildJob[]>([]);
const artifacts = ref<BuildArtifact[]>([]);
const doctorReport = ref<DoctorReport | null>(null);
const doctorRows = ref<Record<string, string>[]>([]);

async function initialize() {
  await load();
  await checkDoctor({ announce: false });
}

async function load() {
  loading.value = true;
  message.value = '';
  try {
    const profileData = await apiGet<BuildProfile[]>('/api/v1/client-build-configs');
    const jobData = await apiGet<BuildJob[]>('/api/v1/build-jobs');
    profiles.value = Array.isArray(profileData) ? profileData : [];
    jobs.value = Array.isArray(jobData) ? jobData : [];
    artifacts.value = artifacts.value.filter((artifact) => jobs.value.some((job) => job.id === artifact.build_job_id));
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    loading.value = false;
  }
}

async function createDefaultProfile() {
  creatingProfile.value = true;
  message.value = '';
  try {
    await apiPost<BuildProfile>('/api/v1/client-build-configs', defaultProfileRequest());
    messageType.value = 'success';
    message.value = 'Build profile created.';
    await load();
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    creatingProfile.value = false;
  }
}

async function queueWindowsJob() {
  if (!profiles.value.length) return;
  queueingJob.value = true;
  message.value = '';
  try {
    await apiPost<BuildJob>(`/api/v1/client-build-configs/${profiles.value[0].id}/jobs`, {
      platform: 'windows_x64'
    });
    messageType.value = 'success';
    message.value = 'Windows job queued.';
    await load();
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    queueingJob.value = false;
  }
}

async function runNextJob() {
  runningJob.value = true;
  message.value = '';
  try {
    const result = await apiPost<{ job: BuildJob; artifacts: unknown[]; message?: string }>('/api/v1/build-jobs/run-next', {});
    messageType.value = result.message ? 'info' : result.job.status === 'failed' ? 'error' : 'success';
    message.value = result.message || `Build job ${result.job.id} finished with ${result.job.status}.`;
    await load();
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    runningJob.value = false;
  }
}

async function refreshDoctor() {
  await checkDoctor({ announce: true });
}

async function checkDoctor(options: { announce: boolean }) {
  checkingDoctor.value = true;
  if (options.announce) {
    message.value = '';
  }
  try {
    const report = await apiGet<DoctorReport>('/api/v1/build-worker/doctor');
    applyDoctorReport(report);
    if (options.announce) {
      messageType.value = report.ready ? 'success' : 'warning';
      message.value = report.ready
        ? 'Windows build environment is ready for the configured mode.'
        : 'Windows build environment needs setup before real artifact builds.';
    }
  } catch (err) {
    doctorReport.value = null;
    doctorRows.value = [];
    messageType.value = 'error';
    message.value = err instanceof Error && !(err instanceof TypeError) ? err.message : humanError(err);
  } finally {
    checkingDoctor.value = false;
  }
}

function applyDoctorReport(report: DoctorReport) {
  if (!report || !Array.isArray(report.checks)) {
    throw new Error('Invalid builder doctor response.');
  }
  doctorReport.value = report;
  doctorRows.value = report.checks.map((check) => ({
    name: check.name,
    required: check.required ? 'yes' : 'no',
    status: check.status,
    detail: check.detail,
    path: check.path || ''
  }));
}

async function cancelJob(job: BuildJob) {
  jobActionId.value = job.id;
  jobAction.value = 'cancel';
  message.value = '';
  try {
    const canceled = await apiPost<BuildJob>(`/api/v1/build-jobs/${job.id}/cancel`, {});
    messageType.value = 'success';
    message.value = `Build job ${canceled.id} canceled.`;
    await load();
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    jobActionId.value = null;
    jobAction.value = '';
  }
}

async function retryJob(job: BuildJob) {
  jobActionId.value = job.id;
  jobAction.value = 'retry';
  message.value = '';
  try {
    const retried = await apiPost<BuildJob>(`/api/v1/build-jobs/${job.id}/retry`, {});
    messageType.value = 'success';
    message.value = `Build job ${retried.id} queued.`;
    await load();
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    jobActionId.value = null;
    jobAction.value = '';
  }
}

async function loadArtifacts(job: BuildJob) {
  jobActionId.value = job.id;
  jobAction.value = 'artifacts';
  message.value = '';
  try {
    artifacts.value = await apiGet<BuildArtifact[]>(`/api/v1/build-jobs/${job.id}/artifacts`);
    messageType.value = artifacts.value.length ? 'success' : 'info';
    message.value = artifacts.value.length
      ? `${artifacts.value.length} artifact(s) available for job ${job.id}.`
      : `No artifacts for job ${job.id}.`;
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    jobActionId.value = null;
    jobAction.value = '';
  }
}

async function downloadArtifact(artifact: BuildArtifact) {
  jobActionId.value = artifact.build_job_id;
  jobAction.value = 'download';
  message.value = '';
  try {
    const blob = await apiDownload(`/api/v1/build-artifacts/${artifact.id}/download`);
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = artifact.filename;
    link.click();
    URL.revokeObjectURL(url);
    messageType.value = 'success';
    message.value = `Downloading ${artifact.filename}.`;
  } catch (err) {
    messageType.value = 'error';
    message.value = humanError(err);
  } finally {
    jobActionId.value = null;
    jobAction.value = '';
  }
}

function defaultProfileRequest() {
  return {
    name: 'Default Windows Profile',
    build_spec: {
      app: {
        name: 'OpenDesk Remote',
        vendor: 'OpenDesk',
        bundle_id: 'com.example.opendeskremote',
        windows_product_name: 'OpenDesk Remote',
        description: 'Open-source self-hosted remote desktop'
      },
      branding: {},
      server: {
        id_server: 'remote.example.com:21116',
        relay_server: 'remote.example.com:21117',
        relay_name: 'hbbr-relay-a',
        api_server: 'https://remote.example.com',
        key: 'PUBLIC_KEY',
        websocket: true,
        relay_grant_required: true
      },
      policy: {
        profile: 'default-secure',
        override_settings: {
          'allow-remote-config-modification': 'N',
          'enable-file-transfer': 'N',
          'enable-terminal': 'N'
        },
        default_settings: {
          'verification-method': 'use-both-passwords'
        }
      },
      platforms: {
        windows_x64: true,
        macos_x64: false,
        macos_arm64: false,
        android_arm64: false,
        ios_arm64: false
      },
      signing: {
        windows: 'unsigned-dev',
        macos: '',
        android: '',
        ios: ''
      },
      source: {
        rustdesk_ref: 'master',
        opendesk_patchset: 'opendesk-remote-m4'
      }
    }
  };
}

onMounted(() => {
  void initialize();
});
</script>
