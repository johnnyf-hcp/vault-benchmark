# Basic Benchmark config options
vault_addr = "http://127.0.0.1:8200"
vault_token = "root"
duration = "2s"
report_mode = "terse"
random_mounts = true

# Test selection and options
test "elasticsearch_secret" "elasticsearch_test_1" {
  weight = 100
  config {
    db_config {
      url      = "https://localhost:9200"
      username = "elastic"
      password = "password"
    }
  }
}