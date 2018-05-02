node {
    stage "pull dockerfiles"
    git branch: 'master', credentialsId: 'github', url: 'https://github.com/akkeris/postgres-rds-prepro.git'

    registry_url    = "http://docker.io"
    docker_creds_id = "docker.io-akkeris"
    org_name        = "akkeris"

    stage "build image"

    docker.withRegistry("${registry_url}", "${docker_creds_id}") {
        build_tag = "1.0.${env.BUILD_NUMBER}"
        container_name = "postgres-rds-prepro"
        container = docker.build("${org_name}/${container_name}:${build_tag}")

        container.push()
        container.push 'latest'
    }
}
