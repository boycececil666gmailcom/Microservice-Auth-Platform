import jenkins.model.Jenkins
import org.jenkinsci.plugins.workflow.job.WorkflowJob
import org.jenkinsci.plugins.workflow.cps.CpsScmFlowDefinition
import hudson.plugins.git.GitSCM
import hudson.plugins.git.BranchSpec
import hudson.triggers.SCMTrigger

// #region Local Pipeline Configuration
def j = Jenkins.get()
def jobName = "url-shortener"

println("[Pipeline-Init] Configuring local pipeline job: " + jobName)
def job = j.getItem(jobName)
if (job == null) {
    job = j.createProject(WorkflowJob.class, jobName)
}

// 1. Point SCM directly to mounted local workspace (zero external network dependency)
def localRepo = "file:///workspace"
def scm = new GitSCM(localRepo)
scm.branches = [new BranchSpec("*")]

// 2. Use Jenkinsfile from the local repository
def flowDefinition = new CpsScmFlowDefinition(scm, "Jenkinsfile")
flowDefinition.lightweight = true
job.definition = flowDefinition

// 3. Set SCM polling to detect local git commits automatically (every minute)
job.addTrigger(new SCMTrigger("* * * * *"))

job.save()
println("[Pipeline-Init] Job successfully configured to track local /workspace")
// #endregion
