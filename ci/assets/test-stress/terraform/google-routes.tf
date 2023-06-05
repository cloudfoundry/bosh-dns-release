resource "google_compute_route" "docker0" {
  name         = "docker0"
  dest_range   = "10.245.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.190"
  tags         = ["docker0"]
}
resource "google_compute_route" "docker1" {
  name         = "docker1"
  dest_range   = "10.246.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.191"
  tags         = ["docker1"]
}
resource "google_compute_route" "docker2" {
  name         = "docker2"
  dest_range   = "10.247.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.192"
  tags         = ["docker2"]
}
resource "google_compute_route" "docker3" {
  name         = "docker3"
  dest_range   = "10.248.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.193"
  tags         = ["docker3"]
}
resource "google_compute_route" "docker4" {
  name         = "docker4"
  dest_range   = "10.249.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.194"
  tags         = ["docker4"]
}
resource "google_compute_route" "docker5" {
  name         = "docker5"
  dest_range   = "10.250.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.195"
  tags         = ["docker5"]
}
resource "google_compute_route" "docker6" {
  name         = "docker6"
  dest_range   = "10.251.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.196"
  tags         = ["docker6"]
}
resource "google_compute_route" "docker7" {
  name         = "docker7"
  dest_range   = "10.252.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.197"
  tags         = ["docker7"]
}
resource "google_compute_route" "docker8" {
  name         = "docker8"
  dest_range   = "10.253.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.198"
  tags         = ["docker8"]
}
resource "google_compute_route" "docker9" {
  name         = "docker9"
  dest_range   = "10.254.0.0/16"
  network      = google_compute_network.bbl-network.name
  next_hop_ip  = "10.0.31.199"
  tags         = ["docker9"]
}