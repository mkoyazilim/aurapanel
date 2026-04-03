package main

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

func (s *service) handleExtendedRoutes(w http.ResponseWriter, r *http.Request) bool {
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/platform/capabilities":
		s.handlePlatformCapabilities(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudlinux/status":
		s.handleCloudLinuxStatus(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudlinux/actions":
		s.handleCloudLinuxActions(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudlinux/actions/run":
		s.handleCloudLinuxActionRun(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudlinux/profiles":
		s.handleCloudLinuxProfiles(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudlinux/rollout/plan":
		s.handleCloudLinuxRolloutPlan(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudlinux/rollout/history":
		s.handleCloudLinuxRolloutHistory(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudlinux/rollout/apply":
		s.handleCloudLinuxRolloutApply(w, r)
		return true
	case r.URL.Path == "/api/v1/terminal/ws":
		if !terminalFeatureEnabled() {
			writeError(w, http.StatusForbidden, "Terminal feature is disabled.")
			return true
		}
		s.handleTerminalWSRoute(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/php/versions":
		s.handlePHPVersions(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/php/extensions":
		s.handlePHPExtensionsList(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/extensions/install":
		s.handlePHPExtensionInstall(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/extensions/remove":
		s.handlePHPExtensionRemove(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/install":
		s.handlePHPInstall(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/remove":
		s.handlePHPRemove(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/restart":
		s.handlePHPRestart(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/ini/get":
		s.handlePHPIniGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/php/ini/save":
		s.handlePHPIniSave(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/websites/advanced-config":
		s.handleWebsiteAdvancedConfigGet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/websites/custom-ssl":
		s.handleWebsiteCustomSSLGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/custom-ssl":
		s.handleWebsiteCustomSSLSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/open-basedir":
		s.handleWebsiteOpenBasedirSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/rewrite":
		s.handleWebsiteRewriteSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/vhost-config":
		s.handleWebsiteVhostConfigSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/subdomains/php":
		s.handleSubdomainPHPSet(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/websites/subdomains":
		s.handleSubdomainDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/subdomains/convert":
		s.handleSubdomainConvert(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/websites/aliases":
		s.handleAliasCreate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/websites/aliases":
		s.handleAliasDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/analytics/website-traffic":
		s.handleWebsiteTraffic(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/dns/zones":
		s.handleDNSZonesList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/dns/zone":
		s.handleDNSZoneCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/dns/reconcile":
		s.handleDNSReconcile(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/dns/default-nameservers":
		s.handleDefaultNameserversGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/dns/default-nameservers":
		s.handleDefaultNameserversSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/dns/default-nameservers/wizard":
		s.handleDefaultNameserversWizard(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/dns/default-nameservers/reset":
		s.handleDefaultNameserversReset(w)
		return true
	case strings.HasPrefix(r.URL.Path, "/api/v1/dns/zones/"):
		s.handleDNSZoneDynamicRoutes(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/list":
		s.handleMailboxesList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/create":
		s.handleMailboxCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/delete":
		s.handleMailboxDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/password":
		s.handleMailboxPassword(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/forwards":
		s.handleMailForwardsList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/forwards":
		s.handleMailForwardCreate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/mail/forwards":
		s.handleMailForwardDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/catch-all":
		s.handleMailCatchAllSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/routing":
		s.handleMailRoutingList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/routing":
		s.handleMailRoutingCreate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/mail/routing":
		s.handleMailRoutingDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/dkim":
		s.handleMailDKIMGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/dkim/rotate":
		s.handleMailDKIMRotate(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/auth":
		s.handleMailAuthGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/auth/bootstrap":
		s.handleMailAuthBootstrap(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/deliverability":
		s.handleMailDeliverability(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/webmail/sso":
		s.handleMailWebmailSSO(w, r)
		return true
	case r.URL.Path == "/api/v1/mail/webmail/sso/consume":
		s.handleMailWebmailConsume(w, r)
		return true
	case r.URL.Path == "/api/v1/mail/webmail/sso/verify":
		s.handleMailWebmailVerify(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/webmail/ops/status":
		s.handleMailWebmailOpsStatus(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/webmail/ops/tokens":
		s.handleMailWebmailOpsTokens(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/webmail/ops/cleanup":
		s.handleMailWebmailOpsCleanup(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/webmail/ops/revoke":
		s.handleMailWebmailOpsRevoke(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/mail/tuning":
		s.handleMailTuningGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/mail/tuning":
		s.handleMailTuningSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ftp/list":
		s.handleTransferList(w, r, "ftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ftp/create":
		s.handleTransferCreate(w, r, "ftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ftp/password":
		s.handleTransferPassword(w, r, "ftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ftp/delete":
		s.handleTransferDelete(w, r, "ftp")
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ftp/tuning":
		s.handleFTPTuningGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ftp/tuning":
		s.handleFTPTuningSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/sftp/list":
		s.handleTransferList(w, r, "sftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sftp/create":
		s.handleTransferCreate(w, r, "sftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sftp/password":
		s.handleTransferPassword(w, r, "sftp")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/sftp/delete":
		s.handleTransferDelete(w, r, "sftp")
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/monitor/cron/jobs":
		s.handleCronJobsList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/monitor/cron/jobs":
		s.handleCronJobCreate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/monitor/cron/jobs":
		s.handleCronJobDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ols/tuning":
		s.handleOLSTuningGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ols/tuning":
		s.handleOLSTuningSet(w, r, false)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ols/tuning/apply":
		s.handleOLSTuningSet(w, r, true)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/storage/minio/buckets":
		s.handleMinIOBucketsList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/storage/minio/buckets":
		s.handleMinIOBucketCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/storage/minio/credentials":
		s.handleMinIOCredentialCreate(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/storage/minio/s3-config":
		s.handleMinIOS3ConfigGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/storage/minio/s3-config":
		s.handleMinIOS3ConfigSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/storage/minio/s3-test":
		s.handleMinIOS3ConfigTest(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/federated/nodes":
		s.handleFederatedNodes(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/federated/mode":
		s.handleFederatedMode(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/federated/join":
		s.handleFederatedJoin(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/files/list":
		s.handleFilesList(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/list":
		s.handleFilesList(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/read":
		s.handleFileRead(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/write":
		s.handleFileWrite(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/rename":
		s.handleFileRename(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/trash":
		s.handleFileTrash(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/delete":
		s.handleFileDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/compress":
		s.handleFileCompress(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/extract":
		s.handleFileExtract(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/files/create_dir":
		s.handleFileCreateDir(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/apps/runtime/list":
		s.handleRuntimeAppsList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/node/install-deps":
		s.handleRuntimeNodeInstall(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/node/start":
		s.handleRuntimeNodeStart(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/node/stop":
		s.handleRuntimeNodeStop(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/python/venv":
		s.handleRuntimePythonVenv(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/python/install":
		s.handleRuntimePythonInstall(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/runtime/python/start":
		s.handleRuntimePythonStart(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/apps/install":
		s.handleCMSInstall(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/sites":
		s.handleWordPressSites(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/scan":
		s.handleWordPressScan(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/plugins":
		s.handleWordPressPluginsGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/plugins/update":
		s.handleWordPressPluginsUpdate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/wordpress/plugins":
		s.handleWordPressPluginsDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/themes":
		s.handleWordPressThemesGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/themes/update":
		s.handleWordPressThemesUpdate(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/wordpress/themes":
		s.handleWordPressThemesDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/backups":
		s.handleWordPressBackupsGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/backups":
		s.handleWordPressBackupCreate(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/backups/download":
		s.handleWordPressBackupDownload(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/backups/restore":
		s.handleWordPressBackupRestore(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/wordpress/staging":
		s.handleWordPressStagingGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/wordpress/staging":
		s.handleWordPressStagingCreate(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/plugins/list":
		s.handlePanelPluginsList(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/plugins/sdk/info":
		s.handlePanelPluginSDKInfo(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plugins/save":
		s.handlePanelPluginSave(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plugins/toggle":
		s.handlePanelPluginToggle(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/plugins/delete":
		s.handlePanelPluginDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/backup/destinations":
		s.handleBackupDestinationsGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/destinations":
		s.handleBackupDestinationSet(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/backup/destinations":
		s.handleBackupDestinationDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/backup/schedules":
		s.handleBackupSchedulesGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/schedules":
		s.handleBackupScheduleSet(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/backup/schedules":
		s.handleBackupScheduleDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/create":
		s.handleBackupCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/snapshots":
		s.handleBackupSnapshots(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/restore":
		s.handleBackupRestore(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/backup/sla/report":
		s.handleBackupSLAReport(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/backup/restore-drill":
		s.handleBackupRestoreDrill(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/backup/restore-drill/history":
		s.handleBackupRestoreDrillHistory(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/backup/list":
		s.handleDBBackupsList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/backup/create":
		s.handleDBBackupCreate(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/backup/download":
		s.handleDBBackupDownload(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/backup/restore":
		s.handleDBBackupRestore(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/backup/delete":
		s.handleDBBackupDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/tools/phpmyadmin/sso":
		s.handleDBToolSSO(w, r, "phpmyadmin")
		return true
	case r.URL.Path == "/api/v1/db/tools/phpmyadmin/sso/consume":
		s.handleDBToolConsume(w, r, "phpmyadmin")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/tools/pgadmin/sso":
		s.handleDBToolSSO(w, r, "pgadmin")
		return true
	case r.URL.Path == "/api/v1/db/tools/pgadmin/sso/consume":
		s.handleDBToolConsume(w, r, "pgadmin")
		return true
	case r.URL.Path == "/api/v1/db/mariadb/remote-access":
		s.handleRemoteAccessCreate(w, r, "mariadb")
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/mariadb/tuning":
		s.handleMariaDBTuningGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/mariadb/tuning":
		s.handleMariaDBTuningSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/db/postgresql/tuning":
		s.handlePostgresTuningGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/db/postgresql/tuning":
		s.handlePostgresTuningSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/activity/log":
		s.handleActivityLog(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/migration/upload":
		s.handleMigrationUpload(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/migration/analyze":
		s.handleMigrationAnalyze(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/migration/import/start":
		s.handleMigrationImportStart(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/migration/import/status":
		s.handleMigrationImportStatus(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/reseller/quotas":
		s.handleResellerQuotasGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/reseller/quotas":
		s.handleResellerQuotaSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/reseller/whitelabel":
		s.handleWhiteLabelsGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/reseller/whitelabel":
		s.handleWhiteLabelSet(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/acl/policies":
		s.handleACLPoliciesGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/acl/policies":
		s.handleACLPolicySet(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/acl/policies":
		s.handleACLPolicyDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/acl/assignments":
		s.handleACLAssignmentsGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/acl/assignments":
		s.handleACLAssignmentSet(w, r)
		return true
	case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/acl/assignments":
		s.handleACLAssignmentDelete(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/acl/effective":
		s.handleACLEffectiveGet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/zones":
		s.handleCloudflareZones(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/cloudflare/status":
		s.handleCloudflareStatus(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/server-auth":
		s.handleCloudflareServerAuth(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/dns/list":
		s.handleCloudflareDNSList(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/dns/create":
		s.handleCloudflareDNSCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/dns/delete":
		s.handleCloudflareDNSDelete(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/ssl":
		s.handleCloudflareSSL(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/security":
		s.handleCloudflareSecurity(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/devmode":
		s.handleCloudflareDevMode(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/cache/purge":
		s.handleCloudflareCachePurge(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/cloudflare/analytics":
		s.handleCloudflareAnalytics(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ssl/bindings":
		s.handleSSLBindings(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ssl/details":
		s.handleSSLDetails(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ssl/hostname/issue":
		s.handleSSLHostnameIssue(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ssl/mail/issue":
		s.handleSSLMailIssue(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ssl/wildcard/issue":
		s.handleSSLWildcardIssue(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/monitor/sre":
		s.handleSREPrediction(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/monitor/sre/log-query":
		s.handleSRELogQuery(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/monitor/sre/optimize":
		s.handleSREOptimize(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/gitops/deploy":
		s.handleGitOpsDeploy(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/perf/redis":
		s.handleRedisIsolationList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/perf/redis":
		s.handleRedisIsolation(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/fail2ban/list":
		s.handleFail2banList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/fail2ban/unban":
		s.handleFail2banUnban(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/ssh/config":
		s.handleSSHConfigGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/ssh/config":
		s.handleSSHConfigSet(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/live-patch":
		s.handleSecurityLivePatch(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/malware/scan/jobs":
		s.handleMalwareJobs(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/malware/scan/status":
		s.handleMalwareStatus(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/malware/scan/start":
		s.handleMalwareStart(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/malware/quarantine":
		s.handleMalwareQuarantine(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/security/malware/quarantine":
		s.handleMalwareQuarantineList(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/security/malware/quarantine/restore":
		s.handleMalwareQuarantineRestore(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docker/containers":
		s.handleDockerContainersGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/containers/create":
		s.handleDockerContainerCreate(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/containers/start":
		s.handleDockerContainerAction(w, r, "start")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/containers/stop":
		s.handleDockerContainerAction(w, r, "stop")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/containers/restart":
		s.handleDockerContainerAction(w, r, "restart")
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/containers/remove":
		s.handleDockerContainerAction(w, r, "remove")
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docker/images":
		s.handleDockerImagesGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/images/pull":
		s.handleDockerImagePull(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/images/remove":
		s.handleDockerImageRemove(w, r)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docker/apps/templates":
		s.handleDockerTemplatesGet(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docker/apps/installed":
		s.handleDockerInstalledAppsGet(w)
		return true
	case r.Method == http.MethodGet && r.URL.Path == "/api/v1/docker/packages":
		s.handleDockerPackagesGet(w)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/apps/install":
		s.handleDockerAppInstall(w, r)
		return true
	case r.Method == http.MethodPost && r.URL.Path == "/api/v1/docker/apps/remove":
		s.handleDockerAppRemove(w, r)
		return true
	default:
		return false
	}
}

func (s *service) handleDNSZoneDynamicRoutes(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/dns/zones/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || strings.TrimSpace(parts[0]) == "" {
		writeError(w, http.StatusNotFound, "Zone not found.")
		return
	}
	domain := normalizeDomain(parts[0])
	switch {
	case len(parts) == 1 && r.Method == http.MethodDelete:
		s.handleDNSZoneDelete(w, domain)
	case len(parts) == 2 && parts[1] == "records" && r.Method == http.MethodGet:
		s.handleDNSRecordsGet(w, domain)
	case len(parts) == 2 && parts[1] == "records" && r.Method == http.MethodPost:
		s.handleDNSRecordCreate(w, r, domain)
	case len(parts) == 2 && parts[1] == "records" && r.Method == http.MethodDelete:
		s.handleDNSRecordDelete(w, r, domain)
	case len(parts) == 2 && parts[1] == "dnssec" && r.Method == http.MethodPost:
		s.handleDNSSECSet(w, r, domain)
	default:
		writeError(w, http.StatusNotFound, "DNS route not found.")
	}
}

func normalizeVirtualPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "/"
	}
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	for strings.Contains(trimmed, "//") {
		trimmed = strings.ReplaceAll(trimmed, "//", "/")
	}
	if len(trimmed) > 1 {
		trimmed = strings.TrimRight(trimmed, "/")
	}
	return trimmed
}

func terminalFeatureEnabled() bool {
	normalized := strings.ToLower(strings.TrimSpace(envOr("AURAPANEL_TERMINAL_ENABLED", "")))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func virtualBaseName(path string) string {
	path = normalizeVirtualPath(path)
	if path == "/" {
		return "/"
	}
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}

func virtualParent(path string) string {
	path = normalizeVirtualPath(path)
	if path == "/" {
		return "/"
	}
	idx := strings.LastIndex(path, "/")
	if idx <= 0 {
		return "/"
	}
	return path[:idx]
}

func (s *service) ensureVirtualDirLocked(path string) {
	key := normalizeVirtualPath(path)
	if item, ok := s.modules.VirtualFiles[key]; ok {
		item.IsDir = true
		return
	}
	s.modules.VirtualFiles[key] = &virtualFile{
		Path:        key,
		IsDir:       true,
		Permissions: "0755",
		ModifiedAt:  time.Now().UTC(),
	}
	parent := virtualParent(key)
	if parent != key {
		if _, ok := s.modules.VirtualFiles[parent]; !ok {
			s.ensureVirtualDirLocked(parent)
		}
	}
}

func (s *service) upsertVirtualFileLocked(path, content, permissions string) {
	key := normalizeVirtualPath(path)
	s.ensureVirtualDirLocked(virtualParent(key))
	s.modules.VirtualFiles[key] = &virtualFile{
		Path:        key,
		IsDir:       false,
		Content:     content,
		Permissions: firstNonEmpty(permissions, "0644"),
		ModifiedAt:  time.Now().UTC(),
	}
}

func (s *service) listVirtualEntriesLocked(path string) []virtualFileEntry {
	dir := normalizeVirtualPath(path)
	items := make([]virtualFileEntry, 0)
	for fullPath, file := range s.modules.VirtualFiles {
		if fullPath == dir {
			continue
		}
		if virtualParent(fullPath) != dir {
			continue
		}
		size := int64(len(file.Content))
		if file.IsDir {
			size = 0
		}
		items = append(items, virtualFileEntry{
			Name:        virtualBaseName(fullPath),
			IsDir:       file.IsDir,
			Size:        size,
			Permissions: firstNonEmpty(file.Permissions, "0644"),
			Modified:    file.ModifiedAt.UnixMilli(),
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return items[i].Name < items[j].Name
	})
	return items
}

func (s *service) deleteVirtualPathLocked(path string) {
	target := normalizeVirtualPath(path)
	for fullPath := range s.modules.VirtualFiles {
		if fullPath == target || strings.HasPrefix(fullPath, target+"/") {
			delete(s.modules.VirtualFiles, fullPath)
		}
	}
}

func (s *service) moveVirtualPathLocked(oldPath, newPath string) {
	oldKey := normalizeVirtualPath(oldPath)
	newKey := normalizeVirtualPath(newPath)
	updates := map[string]*virtualFile{}
	for fullPath, file := range s.modules.VirtualFiles {
		if fullPath == oldKey || strings.HasPrefix(fullPath, oldKey+"/") {
			nextPath := strings.Replace(fullPath, oldKey, newKey, 1)
			clone := *file
			clone.Path = nextPath
			clone.ModifiedAt = time.Now().UTC()
			updates[nextPath] = &clone
			delete(s.modules.VirtualFiles, fullPath)
		}
	}
	s.ensureVirtualDirLocked(virtualParent(newKey))
	for pathKey, file := range updates {
		s.modules.VirtualFiles[pathKey] = file
	}
}

func (s *service) getVirtualFileLocked(path string) (*virtualFile, bool) {
	item, ok := s.modules.VirtualFiles[normalizeVirtualPath(path)]
	return item, ok
}

func writeBlob(w http.ResponseWriter, filename, contentType string, payload []byte) {
	w.Header().Set("Content-Type", firstNonEmpty(contentType, "application/octet-stream"))
	if filename != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}
